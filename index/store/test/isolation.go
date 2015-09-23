package test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/blevesearch/bleve/index/store"
)

// tests focused on verfiying that readers are isolated from writers

func CommonTestReaderIsolation(t *testing.T, s store.KVStore) {
	// insert a kv pair
	writer, err := s.Writer()
	if err != nil {
		t.Error(err)
	}

	// **************************************************
	// this is a hack only required for BoltDB
	// however its harmless so to keep the tests
	// the same everywhere, we include it here
	//
	// this is a hack to try to pre-emptively overflow
	// boltdb writes *MAY* block a long reader
	// in particular, if the write requires additional
	// allocation, it must acquire the same lock as
	// the reader, thus cannot continue until that
	// reader is closed.
	// in general this is not a problem for bleve
	// (though it may affect performance in some cases)
	// but it is a problem for this test which attemps
	// to easily verify that readers are isolated
	// this hack writes enough initial data such that
	// the subsequent writes do not require additional
	// space
	hackSize := 1000
	batch := writer.NewBatch()
	for i := 0; i < hackSize; i++ {
		k := fmt.Sprintf("x%d", i)
		batch.Set([]byte(k), []byte("filler"))
	}
	err = writer.ExecuteBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	// **************************************************

	batch = writer.NewBatch()
	batch.Set([]byte("a"), []byte("val-a"))
	err = writer.ExecuteBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	err = writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	// create an isolated reader
	reader, err := s.Reader()
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err := reader.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	// verify that we see the value already inserted
	val, err := reader.Get([]byte("a"))
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(val, []byte("val-a")) {
		t.Errorf("expected val-a, got nil")
	}

	// verify that an iterator sees it
	count := 0
	it := reader.RangeIterator([]byte{0}, []byte{'x'})
	defer func() {
		err := it.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()
	for it.Valid() {
		it.Next()
		count++
	}
	if count != 1 {
		t.Errorf("expected iterator to see 1, saw %d", count)
	}

	// add something after the reader was created
	writer, err = s.Writer()
	if err != nil {
		t.Error(err)
	}
	batch = writer.NewBatch()
	batch.Set([]byte("b"), []byte("val-b"))
	err = writer.ExecuteBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	err = writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	// ensure that a newer reader sees it
	newReader, err := s.Reader()
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err := newReader.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()
	val, err = newReader.Get([]byte("b"))
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(val, []byte("val-b")) {
		t.Errorf("expected val-b, got nil")
	}

	// ensure that the direct iterator sees it
	count = 0
	it = newReader.RangeIterator([]byte{0}, []byte{'x'})
	defer func() {
		err := it.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()
	for it.Valid() {
		it.Next()
		count++
	}
	if count != 2 {
		t.Errorf("expected iterator to see 2, saw %d", count)
	}

	// but that the isolated reader does not
	val, err = reader.Get([]byte("b"))
	if err != nil {
		t.Error(err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}

	// and ensure that the iterator on the isolated reader also does not
	count = 0
	it = reader.RangeIterator([]byte{0}, []byte{'x'})
	defer func() {
		err := it.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()
	for it.Valid() {
		it.Next()
		count++
	}
	if count != 1 {
		t.Errorf("expected iterator to see 1, saw %d", count)
	}

}
