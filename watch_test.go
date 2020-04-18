package fonctions 

import (
	"log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	passphrase, present := os.LookupEnv("PRIVATE_KEY_PASSPHRASE")
	if !present {
		passphrase = "supersecret"
		log.Printf("Using default passphrase [%v]\n", passphrase)
	}
	config := &Config{
		Interval : 5,
		Watched : "/home/plombard/projects/others/scratchpad/sync",
		// Watched : "/Users/pasca/IdeaProjects/testwebapp/build/libs/",
		Host : "192.168.1.97:9000",
		RemoteRoot : "sync",
		RemoteUser : "access",
		Passphrase : []byte(passphrase),
		Type : "s3",
		Resume: true,
	}
	log.Printf("Using config [%+v]\n", config)
	err := WatchDir(config)
	if err != nil {
		log.Fatal(err)
	}	
}