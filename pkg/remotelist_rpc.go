package remotelist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// --- tipos RPC (exportados) ---
type AppendArgs struct {
	ListID int
	Value  int
}
type AppendReply struct {
	OK bool
}

type GetArgs struct {
	ListID int
	Index  int
}
type GetReply struct {
	Value int
}

type RemoveArgs struct {
	ListID int
}
type RemoveReply struct {
	Value int
}

type SizeArgs struct {
	ListID int
}
type SizeReply struct {
	Size int
}

// --- persistência: log entry e snapshot ---
type LogEntry struct {
	Timestamp int64  `json:"timestamp"`
	Operation string `json:"operation"` //append ou remove
	ListID    int    `json:"list_id"`
	Value     int    `json:"value"` //para append -> valor; para remove -> valor removido
}

type Snapshot struct {
	Timestamp int64         `json:"timestamp"`
	Lists     map[int][]int `json:"lists"`
}

// --- RemoteList ---
type RemoteList struct {
	mu    sync.RWMutex //protege acesso a lists
	lists map[int][]int

	//locks por lista
	locksMu   sync.Mutex
	listLocks map[int]*sync.Mutex

	//snapshot vs handlers
	snapshotRW sync.RWMutex

	// rquivos
	basePath      string
	logFile       string
	snapshotFile  string
	logMutex      sync.Mutex
	snapshotMutex sync.Mutex
}

// --- Configuração de arquivos de persistência ---
func NewRemoteListWithBase(basePath string) *RemoteList {
	rl := &RemoteList{
		lists:        make(map[int][]int),
		listLocks:    make(map[int]*sync.Mutex),
		basePath:     basePath,
		logFile:      basePath + ".log",
		snapshotFile: basePath + ".snapshot",
	}
	return rl
}

// --- utilitário JSONL scanner (simples) ---
type JSONLScanner struct {
	data  []byte
	start int
}

func NewJSONLScanner(data []byte) *JSONLScanner {
	return &JSONLScanner{data: data}
}
func (s *JSONLScanner) Scan() bool {
	for s.start < len(s.data) && s.data[s.start] == '\n' {
		s.start++
	}
	return s.start < len(s.data)
}
func (s *JSONLScanner) Bytes() []byte {
	end := s.start
	for end < len(s.data) && s.data[end] != '\n' {
		end++
	}
	result := s.data[s.start:end]
	s.start = end + 1
	return result
}

// --- obtém (ou cria) mutex por lista ---
func (rl *RemoteList) getListLock(listID int) *sync.Mutex {
	rl.locksMu.Lock()
	l, ok := rl.listLocks[listID]
	if !ok {
		l = &sync.Mutex{}
		rl.listLocks[listID] = l
	}
	rl.locksMu.Unlock()
	return l
}

// --- appendToLog (thread-safe) ---
func (rl *RemoteList) appendToLog(op string, listID int, value int) error {
	rl.logMutex.Lock()
	defer rl.logMutex.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().UnixNano(),
		Operation: op,
		ListID:    listID,
		Value:     value,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// abrir, append, fechar
	f, err := os.OpenFile(rl.logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	_ = f.Close()
	return err
}

// --- CreateSnapshot (gera snapshot atômico) ---
func (rl *RemoteList) CreateSnapshot() error {
	rl.snapshotRW.Lock()
	defer rl.snapshotRW.Unlock()

	rl.snapshotMutex.Lock()
	defer rl.snapshotMutex.Unlock()

	rl.mu.RLock()
	copyLists := make(map[int][]int, len(rl.lists))
	for k, v := range rl.lists {
		c := make([]int, len(v))
		copy(c, v)
		copyLists[k] = c
	}
	rl.mu.RUnlock()

	snap := Snapshot{
		Timestamp: time.Now().UnixNano(),
		Lists:     copyLists,
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(rl.snapshotFile)
	tmpFile := filepath.Join(dir, fmt.Sprintf(".%s.tmp", filepath.Base(rl.snapshotFile)))
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, rl.snapshotFile)
}

// --- LoadFromSnapshot (snapshot + replay log) ---
func (rl *RemoteList) LoadFromSnapshot() error {
	// no arranque, garantimos que nenhuma goroutine handler está ativa ainda,
	// mas por segurança vamos bloquear snapshotRW enquanto carregamos.
	rl.snapshotRW.Lock()
	defer rl.snapshotRW.Unlock()

	rl.snapshotMutex.Lock()
	defer rl.snapshotMutex.Unlock()

	var snapTS int64 = 0

	//carregar snapshot se existir
	if b, err := os.ReadFile(rl.snapshotFile); err == nil {
		var snap Snapshot
		if err := json.Unmarshal(b, &snap); err == nil {
			rl.mu.Lock()
			rl.lists = make(map[int][]int, len(snap.Lists))
			for k, v := range snap.Lists {
				c := make([]int, len(v))
				copy(c, v)
				rl.lists[k] = c
			}
			rl.mu.Unlock()
			snapTS = snap.Timestamp
			fmt.Printf("[Load] Snapshot carregado (timestamp=%d), %d listas.\n", snap.Timestamp, len(snap.Lists))
		} else {
			fmt.Println("[Load] Erro ao unmarshal snapshot:", err)
		}
	} else {
		fmt.Println("[Load] Nenhum snapshot encontrado, iniciando com mapa vazio")
	}

	//replay do log (aplica apenas operações posteriores ao snapshot)
	if b, err := os.ReadFile(rl.logFile); err == nil {
		scanner := NewJSONLScanner(b)
		for scanner.Scan() {
			var entry LogEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				//ignorar entradas corrompidas
				continue
			}
			//pular entradas já incorporadas no snapshot
			if snapTS != 0 && entry.Timestamp <= snapTS {
				continue
			}
			rl.applyLogEntry(entry, false /* já está persistido no log */)
		}
		fmt.Println("[Load] Replay do log concluído")
	} else {

	}

	return nil
}

// --- applyLogEntry (aux) ---
func (rl *RemoteList) applyLogEntry(entry LogEntry, logWritten bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	switch entry.Operation {
	case "append":
		rl.lists[entry.ListID] = append(rl.lists[entry.ListID], entry.Value)
	case "remove":
		if ls, ok := rl.lists[entry.ListID]; ok && len(ls) > 0 {
			rl.lists[entry.ListID] = ls[:len(ls)-1]
		}
	}
	_ = logWritten
}

// --- RPC Methods (exported) ---

// Append: adiciona value ao final da lista list_id
func (rl *RemoteList) Append(args AppendArgs, reply *AppendReply) error {
	//evitar conflito com snapshot (muitos handlers podem operar quando não há snapshot)
	rl.snapshotRW.RLock()
	defer rl.snapshotRW.RUnlock()

	//lock específico da lista
	lck := rl.getListLock(args.ListID)
	lck.Lock()
	defer lck.Unlock()

	//gravar no log primeiro (WAL-like) para durabilidade
	if err := rl.appendToLog("append", args.ListID, args.Value); err != nil {
		return err
	}

	//aplicar em memória
	rl.mu.Lock()
	rl.lists[args.ListID] = append(rl.lists[args.ListID], args.Value)
	rl.mu.Unlock()

	reply.OK = true
	return nil
}

// Get: retorna o valor na posição i da lista list_id
func (rl *RemoteList) Get(args GetArgs, reply *GetReply) error {
	rl.snapshotRW.RLock()
	defer rl.snapshotRW.RUnlock()

	lck := rl.getListLock(args.ListID)
	lck.Lock()
	defer lck.Unlock()

	rl.mu.RLock()
	ls, ok := rl.lists[args.ListID]
	rl.mu.RUnlock()
	if !ok {
		return errors.New("lista não existe")
	}
	if args.Index < 0 || args.Index >= len(ls) {
		return errors.New("índice fora do intervalo")
	}
	reply.Value = ls[args.Index]
	return nil
}

// Remove: remove e retorna o último elemento da lista list_id
func (rl *RemoteList) Remove(args RemoveArgs, reply *RemoveReply) error {
	rl.snapshotRW.RLock()
	defer rl.snapshotRW.RUnlock()

	lck := rl.getListLock(args.ListID)
	lck.Lock()
	defer lck.Unlock()

	rl.mu.Lock()
	ls, ok := rl.lists[args.ListID]
	if !ok || len(ls) == 0 {
		rl.mu.Unlock()
		return errors.New("lista vazia ou não existente")
	}
	val := ls[len(ls)-1]
	rl.lists[args.ListID] = ls[:len(ls)-1]
	rl.mu.Unlock()

	//gravar no log o valor removido (durability)
	if err := rl.appendToLog("remove", args.ListID, val); err != nil {
		//se falhar ao logar, isso é grave — mas já removemos em memória.
		//olítica: reportar erro para o cliente (pode decidir re-tentar)
		return fmt.Errorf("erro ao gravar log de remove: %w", err)
	}

	reply.Value = val
	return nil
}

// Size: retorna a quantidade de elementos da lista list_id
func (rl *RemoteList) Size(args SizeArgs, reply *SizeReply) error {
	rl.snapshotRW.RLock()
	defer rl.snapshotRW.RUnlock()

	lck := rl.getListLock(args.ListID)
	lck.Lock()
	defer lck.Unlock()

	rl.mu.RLock()
	ls := rl.lists[args.ListID]
	rl.mu.RUnlock()
	reply.Size = len(ls)
	return nil
}

// GetLists (apenas para debug/testing) - retorna cópia
func (rl *RemoteList) GetLists(_ struct{}, reply *map[int][]int) error {
	rl.snapshotRW.RLock()
	defer rl.snapshotRW.RUnlock()

	rl.mu.RLock()
	defer rl.mu.RUnlock()
	out := make(map[int][]int, len(rl.lists))
	for k, v := range rl.lists {
		c := make([]int, len(v))
		copy(c, v)
		out[k] = c
	}
	*reply = out
	return nil
}
