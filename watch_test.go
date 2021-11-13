package fonctions

import (
	"flag"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	passphrase, present := os.LookupEnv("PRIVATE_KEY_PASSPHRASE")
	var watchedDir, host, root string
	var resume bool
	flag.StringVar(&watchedDir, "watch", "/home/plombard/projects/others/scratchpad/sync", "directory to watch")
	flag.StringVar(&host, "host", "192.168.1.50:9000", "s3 host:port")
	flag.StringVar(&root, "root", "sync", "directory on the host to copy to")
	flag.BoolVar(&resume, "resume", true, "resume or reset (wipe the root dir on the host) sync")
	flag.Parse()
	if !present {
		passphrase = "minioadmin"
		log.Printf("Using default passphrase [%v]\n", passphrase)
	}
	config := &Config{
		Interval : 5,
		Watched : watchedDir,
		// Watched : "/Users/pasca/IdeaProjects/testwebapp/build/libs/",
		Host : host,
		RemoteRoot : root,
		RemoteUser : "minioadmin",
		Passphrase : []byte(passphrase),
		Type : "s3",
		Resume: resume,
	}
	log.SetLevel(log.DebugLevel)
	log.Printf("Using config [%+v]\n", config)
	err := WatchDir(config)
	if err != nil {
		log.Fatal(err)
	}	
}