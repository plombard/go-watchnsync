package fonctions

import (
	"crypto/md5"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"code.cloudfoundry.org/bytefmt"
	"github.com/minio/minio-go"
	"github.com/pkg/sftp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// Clear Vide (supprime puis recrée) le repertoire distant.
func Clear(config *Config) error {
	if config.Type == "s3" {
		s3Client, errconn := connectUsingMinio(config)
		if errconn != nil {
			log.Errorf("Problème lors de la connection à [%s]\n", config.Host)
			return errconn
		}

		var toDelete []string

		// Send object names that are needed to be removed to objectsCh
		go func() {
			doneCh := make(chan struct{})
			// Indicate to our routine to exit cleanly upon return.
			defer close(doneCh)
			// List all objects from a bucket-name with a matching prefix.
			for object := range s3Client.ListObjectsV2(config.RemoteRoot, "", true, doneCh) {
				if object.Err != nil {
					log.Fatalln(object.Err)
				}
				markedForDeletion := false
				// Mark the object for deletion,
				// but in the case of a resuming operation,
				// only if it's older than an existing local counterpart
				if !config.Resume {
					markedForDeletion = true
				} else {
					// look for an older local counterpart
					srcFilename := filepath.Join(config.Watched, object.Key)
					// localCounterPartExistsAndIsOlder := fileExistsAndIsOlder(srcFilename, object.LastModified)
					localCounterPartExistsAndIsDifferent := fileExistsAndIsDifferent(srcFilename, strings.Trim(object.ETag, "\""))
					// do remove file only if resume mode is active,
					// and local file is different (hence not already copied)
					if !localCounterPartExistsAndIsDifferent {
						markedForDeletion = true
					} else {
						log.Printf("No local file [%v] or local [%s] checksum is different than ETag[%v]\n", object.Key, srcFilename, strings.Trim(object.ETag, "\""))
					}
				}
				if markedForDeletion {
					toDelete = append(toDelete, object.Key)
				}
			}
		}()

		for _, object := range toDelete {
			rErr := s3Client.RemoveObject(config.RemoteRoot, object)
			if rErr != nil {
				fmt.Println("Error detected during deletion: ", rErr)
				return rErr
			}
		}
		
		if config.Resume {
			log.Printf("Répertoire distant [%s] nettoyé\n", config.RemoteRoot)
		} else {
			log.Printf("Répertoire distant [%s] remis à zéro\n", config.RemoteRoot)
		}		
		return nil
	}

	session, errconn := connectUsingKey(config)
	if errconn != nil {
		log.Errorf("Problème lors de la connection à [%s]\n", config.Host)
		return errconn
	}
	defer session.Close()

	// Read directory
	var later []string
	remotePath := filepath.Join(config.RemoteRoot, filepath.Base(config.Watched))
	walker := session.Walk(remotePath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			if os.IsNotExist(err) {
				continue
			} else {
				return err
			}
		}
		filename := walker.Path()
		if walker.Stat().IsDir() {
			later = append(later, filename)
			log.Debugf("Found dir [%s]", filename)
		} else {
			log.Debugf("Found [%s]", filename)
			errDel := session.Remove(filename)
			if errDel != nil {
				return errDel
			}
			log.Infof("Deleted [%s]", filename)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(later)))
	for _, path := range later {
		errDel := session.RemoveDirectory(path)
		if errDel != nil {
			return errDel
		}
		log.Infof("Deleted dir [%s]", path)
	}

	// At this point, remote target dir might be nonexistent,
	// try to create it just in case.
	errNew := session.MkdirAll(remotePath)
	if errNew != nil {
		return errNew
	}
	log.Printf("Répertoire distant [%s] remis à zéro\n", remotePath)

	return nil
}

func isEmptyDir(name string) (bool, error) {
	entries, err := ioutil.ReadDir(name)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

// fileExistsAndIsDifferent checks if a file exists and has diferrent checksum than the one provided.
func fileExistsAndIsDifferent(filename string, checksum string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	// To prevent errors with directories modified times,
	// consider never removing existing directories
	if info.IsDir() {
		return false
	}
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	calculatedHash := md5.New()
	if _, err := io.Copy(calculatedHash, f); err != nil {
		log.Fatal(err)
	}
	log.Debugf("File[%s] s3hash[%s] calculated[%s] equals[%t]", filename, checksum, hex.EncodeToString(calculatedHash.Sum(nil)), strings.EqualFold(checksum, hex.EncodeToString(calculatedHash.Sum(nil))))
	return !strings.EqualFold(checksum, hex.EncodeToString(calculatedHash.Sum(nil)))
}

// fileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Upload telecharge les fichiers en amont.
func Upload(config *Config, ajouter, supprimer, ajouterd []string) error {
	remotePath := filepath.Base(config.Watched)
	log.Infof("Remote path : [%s]", remotePath)

	if config.Type == "s3" {
		s3Client, err := connectUsingMinio(config)
		if err != nil {
			return err
		}

		objectsCh := make(chan string)
		sort.Sort(sort.Reverse(sort.StringSlice(supprimer)))
		go func() {
			defer close(objectsCh)
			for _, source := range supprimer {
				// dstFilename := filepath.Join(remotePath, source)
				dstFilename := source
				if dstFilename != "." {
					log.Infof("Object to remove [%s]\n", dstFilename)
					objectsCh <- dstFilename
				}
			}
		}()

		for rErr := range s3Client.RemoveObjects(config.RemoteRoot, objectsCh) {
			log.Error("Error detected during deletion: ", rErr)
		}

		for _, source := range ajouterd {
			// Ce cas n'est pas traité par Minio
			srcDirname := filepath.Join(config.Watched, source)
			// dstDirname := filepath.Join(remotePath, source)
			dstDirname := source
			isEmpty, err := isEmptyDir(srcDirname)
			if err != nil {
				return err
			}
			if isEmpty {
				log.Warningf("Creation of empty remote directory [%s] is not allowed", dstDirname)
			}
		}
		for index, source := range ajouter {
			// create source file
			srcFilename := filepath.Join(config.Watched, source)
			// dstFilename := filepath.Join(remotePath, source)
			dstFilename := source
			var doCopy bool
			if config.Resume {
				objectInfo, err := s3Client.StatObject(config.RemoteRoot, dstFilename, minio.StatObjectOptions{})
				if err == nil {
					doCopy = fileExistsAndIsDifferent(srcFilename, objectInfo.ETag)
				} else {
					if strings.Contains(err.Error(), "does not exist.") {
						doCopy = true
					} else {
						return err
					}	
				}
			} else {
				doCopy = fileExists(srcFilename)
			}
			if doCopy {
				bytes, err := s3Client.FPutObject(config.RemoteRoot, dstFilename, srcFilename, minio.PutObjectOptions{})
				if err != nil {
					return err
				}
				log.Infof("[%s] remotely copied for [%s] [file %d/%d]\n", bytefmt.ByteSize(uint64(bytes)), dstFilename, index + 1, len(ajouter))
			} else {
				log.Infof("Skipping object [%s] (already existing) [file %d/%d]\n", dstFilename, index + 1, len(ajouter))
			}
		}

		return nil
	}

	client, err := connectUsingKey(config)
	if err != nil {
		return err
	}
	defer client.Close()

	for _, source := range ajouterd {
		// create remote directory
		path := filepath.Join(remotePath, source)
		err := client.MkdirAll(path)
		if err != nil {
			return err
		}
		log.Infof("Created remote directory [%s]", path)
	}
	for _, source := range ajouter {
		// create source file
		srcFile, err := os.Open(filepath.Join(config.Watched, source))
		if err != nil {
			return err
		}

		// create destination file
		dstFile, err := client.Create(filepath.Join(remotePath, source))
		if err != nil {
			return err
		}
		defer dstFile.Close()

		// copy source file to destination file
		bytes, err := io.Copy(dstFile, srcFile)
		if err != nil {
			return err
		}
		log.Infof("[%s] remotely copied for file [%s]\n", bytefmt.ByteSize(uint64(bytes)), source)
	}

	// Trick : on supprime dans l'ordre decroissant
	// pour être sûr que les fichiers d'un repertoire
	// soient supprimés avant celui-ci
	sort.Sort(sort.Reverse(sort.StringSlice(supprimer)))
	for _, source := range supprimer {
		// delete remote file
		path := filepath.Join(remotePath, source)
		err := client.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		log.Infof("Deleted remotely [%s]\n", path)
	}

	return nil
}

func connectUsingKey(config *Config) (*sftp.Client, error) {
	// If you have an encrypted private key, the crypto/x509 package
	// can be used to decrypt it.
	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf("unable to get current user: %v", err)
	}
	filename := config.KeyFile
	if filename == "" {
		filename = filepath.Join(currentUser.HomeDir, ".ssh", "id_rsa")
	}
	key, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	block, _ := pem.Decode(key)
	if block == nil {
		log.Fatalf("unable to decode private key\n")
	}

	decBlock, err := x509.DecryptPEMBlock(block, config.Passphrase)
	if err != nil {
		log.Fatalf("unable to decrypt private key\n")
	}

	parsedKey, err := x509.ParsePKCS1PrivateKey(decBlock)
	if err != nil {
		log.Fatalf("unable to parse private key [%s]: %v", block.Type, err)
	}
	signer, err := ssh.NewSignerFromKey(parsedKey)

	// Create the Signer for this private key.
	// signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: config.RemoteUser,
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", config.Host+":22", sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: [%s]", err.Error())
	}
	log.Info("Successfully connected to ssh server.")

	// open an SFTP session over an existing ssh connection.
	session, err := sftp.NewClient(client)
	if err != nil {
		return nil, err
	}
	log.Info("Successfully initiated sftp session.")
	return session, nil
}

func connectUsingMinio(config *Config) (*minio.Client, error) {
	// Initialize minio client object.
	endpoint := config.Host
	accessKeyID := config.RemoteUser
	secretAccessKey := string(config.Passphrase[:])
	useSSL := false
	minioClient, err := minio.New(endpoint, accessKeyID, secretAccessKey, useSSL)
	if err != nil {
		return nil, err
	}

	found, err := minioClient.BucketExists(config.RemoteRoot)
	if err != nil {
		return nil, err
	}
	if !found {
		err = minioClient.MakeBucket(config.RemoteRoot, "")
		if err != nil {
			return nil, err
		}
		log.Infof("Successfully created bucket [%s]", config.RemoteRoot)
	}

	return minioClient, nil
}