package masterserver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matarc/filewatcher/shared"

	"github.com/boltdb/bolt"
	"github.com/matarc/filewatcher/log"
)

func TestInit(t *testing.T) {
	srv := new(Server)
	srv.Init()
	if srv.Address != shared.DefaultMasterserverAddress {
		t.Fatalf("address should be '%s', instead is '%s'", shared.DefaultMasterserverAddress, srv.Address)
	}
	if srv.StorageAddress != shared.DefaultStorageAddress {
		t.Fatalf("storageAddress should be '%s', instead is '%s'", shared.DefaultStorageAddress, srv.StorageAddress)
	}

	srv = new(Server)
	srv.Address = "localhost:12345"
	srv.StorageAddress = "localhost:54321"
	srv.Init()
	if srv.Address != "localhost:12345" {
		t.Fatalf("address should be 'localhost:12345', instead is '%s'", srv.Address)
	}
	if srv.StorageAddress != "localhost:54321" {
		t.Fatalf("storageAddress should be 'localhost:54321', instead is '%s'", srv.StorageAddress)
	}
}

func Test_getList(t *testing.T) {
	// Setup
	rootDir, err := ioutil.TempDir("", "filewatcher")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootDir)
	dbPath := filepath.Join(rootDir, "mydb")
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Error(err)
		return
	}
	defer db.Close()
	err = db.Batch(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("1"))
		if err != nil {
			return err
		}
		err = b.Put([]byte("/my/b"), []byte{})
		if err != nil {
			return err
		}
		return b.Put([]byte("/my/a"), []byte{})
	})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", shared.DefaultStorageAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	rpcSrv := rpc.NewServer()
	paths := new(shared.Paths)
	paths.Db = db
	rpcSrv.Register(paths)
	go rpcSrv.Accept(listener)
	srv := new(Server)
	shared.LoadConfig("", srv)

	// Test
	nodes, err := srv.getList()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes should have '1' node, instead has '%d'", len(nodes))
	}
	if len(nodes[0].Files) != 2 {
		t.Fatalf("First node should have '2' path, instead has '%d'", len(nodes[0].Files))
	}
	if nodes[0].Files[0] != "/my/a" {
		t.Fatalf("First file should be '/my/a', instead is '%s'", nodes[0].Files[0])
	}
	if nodes[0].Files[1] != "/my/b" {
		t.Fatalf("First file should be '/my/b', instead is '%s'", nodes[0].Files[0])
	}
}

func TestSendList(t *testing.T) {
	// Setup
	rootDir, err := ioutil.TempDir("", "filewatcher")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootDir)
	dbPath := filepath.Join(rootDir, "mydb")
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Error(err)
		return
	}
	defer db.Close()
	err = db.Batch(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("1"))
		if err != nil {
			return err
		}
		err = b.Put([]byte("/my/b"), []byte{})
		if err != nil {
			return err
		}
		return b.Put([]byte("/my/a"), []byte{})
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := new(Server)
	shared.LoadConfig("", srv)
	err = srv.Run()
	defer srv.Stop()
	if err != nil {
		t.Fatal(err)
	}

	// Test
	res, err := http.Get(fmt.Sprintf("http://%s/list", shared.DefaultMasterserverAddress))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadGateway {
		t.Fatalf("Status should be '%d', instead is '%s'", http.StatusBadGateway, res.Status)
	}
	res.Body.Close()

	// Additional setup for next test
	listener, err := net.Listen("tcp", shared.DefaultStorageAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	rpcSrv := rpc.NewServer()
	paths := new(shared.Paths)
	paths.Db = db
	rpcSrv.Register(paths)
	go rpcSrv.Accept(listener)

	// Test
	res, err = http.Get(fmt.Sprintf("http://%s/list", shared.DefaultMasterserverAddress))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Status should be '%d', instead is '%s'", http.StatusOK, res.Status)
	}
	nodes := []shared.Node{}
	err = json.NewDecoder(res.Body).Decode(&nodes)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if len(nodes) != 1 {
		t.Fatalf("nodes should have '1' node, instead has '%d'", len(nodes))
	}
	if len(nodes[0].Files) != 2 {
		t.Fatalf("First node should have '2' path, instead has '%d'", len(nodes[0].Files))
	}
	if nodes[0].Files[0] != "/my/a" {
		t.Fatalf("First file should be '/my/a', instead is '%s'", nodes[0].Files[0])
	}
	if nodes[0].Files[1] != "/my/b" {
		t.Fatalf("First file should be '/my/b', instead is '%s'", nodes[0].Files[0])
	}

	listener.Close()
	// Test cache
	res, err = http.Get(fmt.Sprintf("http://%s/list", shared.DefaultMasterserverAddress))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Status should be '%d', instead is '%s'", http.StatusOK, res.Status)
	}
	nodes = []shared.Node{}
	err = json.NewDecoder(res.Body).Decode(&nodes)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if len(nodes) != 1 {
		t.Fatalf("nodes should have '1' node, instead has '%d'", len(nodes))
	}
	if len(nodes[0].Files) != 2 {
		t.Fatalf("First node should have '2' path, instead has '%d'", len(nodes[0].Files))
	}
	if nodes[0].Files[0] != "/my/a" {
		t.Fatalf("First file should be '/my/a', instead is '%s'", nodes[0].Files[0])
	}
	if nodes[0].Files[1] != "/my/b" {
		t.Fatalf("First file should be '/my/b', instead is '%s'", nodes[0].Files[0])
	}

	// Test cache again (should still be fine)
	res, err = http.Get(fmt.Sprintf("http://%s/list", shared.DefaultMasterserverAddress))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Cache wasn't saved, Status should be '%d', instead is '%s'", http.StatusOK, res.Status)
	}
}
