package fonctions

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

var toUpload []string
var toDelete []string
var toUploadd []string
var actuels []string
var actuelsd []string
var rootpath string
var watcher *fsnotify.Watcher

// Config stores all the configuration for dir watched and remote syncing.
type Config struct {
	Watched string
	Host string
	RemoteRoot string
	RemoteUser string
	Interval time.Duration
	KeyFile string
	Passphrase []byte
	Type string
	Resume bool // Skip deleting the remote path first
}

// WatchDir Tous les interval, recupere et synchronise les fichiers modifiés du repertoire watched.
func WatchDir(config *Config) error {
	rootpath = config.Watched
	// Ecoute les signaux, et notifie le ctrl-c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		// Block until a signal is received.
		s := <-c
		log.Println("Got signal:", s)
		os.Exit(0)
	}()

	log.Infof("Base path : [%s]", filepath.Base(rootpath))
	err := Clear(config)
	if err != nil {
		return err
	}

	
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Cannot create fsnotify", err)
	}
	defer watcher.Close()

	err = filepath.Walk(config.Watched, visit)
	if err != nil {
		return fmt.Errorf("filepath.Walk() returned %v", err)
	}
	
	err = Upload(config, actuels, nil, actuelsd)
	if err != nil {
		return fmt.Errorf("Upload() returned %v", err)
	}

	ticker := time.NewTicker(config.Interval * time.Second)
	go func() {
		for range ticker.C {
			timedLoop(config)
		}
	}()
	defer ticker.Stop()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Calcul de l'emplacement du fichier par rapport au répertoire observé
				relative, relerr := filepath.Rel(rootpath, event.Name)
				if relerr != nil {
					log.Warningf("Cannot process filepath [%s] : [%v]", event.Name, relerr)
					return
				}

				switch event.Op {
				case fsnotify.Write, fsnotify.Create, fsnotify.Rename:
					log.Debugf("modified/created [%v]", event)
					info, err := os.Stat(event.Name)
					if err != nil {
						log.Warningf("Cannot stat on [%s] : [%v]", event.Name, err)
						break
					}
					watcher.Add(event.Name)
					if info.IsDir() {
						toUploadd = append(toUploadd, relative)
					} else {
						toUpload = append(toUpload, relative)
					}
				case fsnotify.Remove:
					log.Debugf("removed [%v]\n", event)
					toDelete = append(toDelete, relative)
					watcher.Remove(event.Name)
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error("error:", err)
			}
		}
	}()

	err = watcher.Add(config.Watched)
	if err != nil {
		log.Fatalf("Cannot add watcher on [%s] : [%v]", config.Watched, err)
	}
	<-done


	return nil
}

func visit(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	log.Debugf("Visited: [%s] [%s]", path, info.ModTime())
	relative, relerr := filepath.Rel(rootpath, path)
	if relerr != nil || relative == "." {
		return relerr
	}
	if info.IsDir() {
		actuelsd = append(actuelsd, relative)
		return watcher.Add(path)
	}
	actuels = append(actuels, relative)

	return err
}

func timedLoop(config *Config) {
	toUpload = prune(toUpload)
	toUploadd = prune(toUploadd)
	toDelete = prune(toDelete)
	toUploadd = remove(toUploadd, ".")
	// toUpload, toDelete = removeIntersec(toUpload, toDelete)
	// toUploadd, toDelete = removeIntersec(toUploadd, toDelete)
	if len(toUploadd) > 0 || len(toUpload) > 0 || len(toDelete) > 0 {
		log.Info("")
		log.Infof("Dirs to upload [%v]", toUploadd)
		log.Infof("Files to upload [%v]", toUpload)
		log.Infof("Files to delete [%v]", toDelete)
		log.Info("")
		err := Upload(config, toUpload, toDelete, toUploadd)
		if err != nil {
			log.Fatalf("Upload() returned [%v]", err)
		}
	}
	toUploadd = nil
	toUpload = nil
	toDelete = nil
}
