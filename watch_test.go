package fonctions 

import (
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	passphrase, present := os.LookupEnv("PRIVATE_KEY_PASSPHRASE")
	if !present {
		passphrase = "minioadmin"
		log.Printf("Using default passphrase [%v]\n", passphrase)
	}
	config := &Config{
		Interval : 5,
		Watched : "/home/user/tmp/sync/local",
		// Watched : "/Users/pasca/IdeaProjects/testwebapp/build/libs/",
		Host : "localhost:9000",
		RemoteRoot : "remote",
		RemoteUser : "minioadmin",
		Passphrase : []byte(passphrase),
		Type : "s3",
		Resume: true,
	}
	log.SetLevel(log.DebugLevel)
	log.Printf("Using config [%+v]\n", config)
	err := WatchDir(config)
	if err != nil {
		log.Fatal(err)
	}	
}