package remotelist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

type RemoteList struct {
	mu   sync.Mutex
	list []int
	size uint32
}

var persistFile string

// SetPersistFile define o arquivo onde a lista será persistida.
// dseve ser chamada pelo servidor após criar a RemoteList.
func SetPersistFile(path string) {
	persistFile = path
}

// LoadFrom carrega a lista do arquivo informado (se existir).
func (l *RemoteList) LoadFrom(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var items []int
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list = items
	l.size = uint32(len(items))
	return nil
}

// saveLocked grava a lista em JSON no arquivo configurado.
// assume que l.mu já está travado.
func (l *RemoteList) saveLocked() error {
	if persistFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(l.list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(persistFile, data, 0644)
}

func (l *RemoteList) Append(value int, reply *bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list = append(l.list, value)
	fmt.Println(l.list)
	l.size++
	*reply = true

	if err := l.saveLocked(); err != nil {
		// logar, mas não falhar a operação RPC
		fmt.Println("erro ao persistir:", err)
	}
	return nil
}

func (l *RemoteList) Remove(arg int, reply *int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.list) > 0 {
		*reply = l.list[len(l.list)-1]
		l.list = l.list[:len(l.list)-1]
		fmt.Println(l.list)
	} else {
		return errors.New("empty list")
	}

	if err := l.saveLocked(); err != nil {
		fmt.Println("erro ao persistir:", err)
	}
	return nil
}

func (l *RemoteList) Get(index int, reply *int) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if index < 0 || index >= len(l.list) {
		return errors.New("índice fora do intervalo")
	}

	*reply = l.list[index]
	return nil
}

func (l *RemoteList) Size(_ int, reply *int) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	*reply = len(l.list)
	return nil
}

// retorna uma cópia da lista atual de inteiros.
func (l *RemoteList) GetList() []int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.list
}

func NewRemoteList() *RemoteList {
	return new(RemoteList)
}
