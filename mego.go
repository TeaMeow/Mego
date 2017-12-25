package mego

import (
	"errors"
	"net/http"

	uuid "github.com/satori/go.uuid"
	"github.com/vmihailenco/msgpack"

	"github.com/olahol/melody"
)

const (
	// 狀態碼範圍如下：
	// 0 ~ 50 正常、51 ~ 100 錯誤、101 ~ 999 自訂狀態碼。

	// StatusOK 表示正常。
	StatusOK = 0
	// StatusProcessing 表示請求已被接受並且正在處理中，並不會立即完成。
	StatusProcessing = 1
	// StatusNoChanges 表示這個請求沒有改變任何結果，例如：使用者刪除了一個早已被刪除的物件。
	StatusNoChanges = 2
	// StatusFileNext 表示此檔案區塊已處理完畢，需上傳下個區塊。
	StatusFileNext = 10
	// StatusFileAbort 表示終止整個檔案上傳進度。
	StatusFileAbort = 11

	// StatusError 表示有內部錯誤發生。
	StatusError = 51
	// StatusFull 表示此請求無法被接受，因為額度已滿。例如：使用者加入了一個已滿的聊天室、好友清單已滿。
	StatusFull = 52
	// StatusExists 表示請求的事物已經存在，例如：重複的使用者名稱、電子郵件地址。
	StatusExists = 53
	// StatusInvalid 表示此請求格式不正確。
	StatusInvalid = 54
	// StatusNotFound 表示找不到請求的資源。
	StatusNotFound = 55
	// StatusNotAuthorized 表示使用者需要登入才能進行此請求。
	StatusNotAuthorized = 56
	// StatusNoPermission 表示使用者已登入但沒有相關權限執行此請求。
	StatusNoPermission = 57
	// StatusUnimplemented 表示此功能尚未實作完成。
	StatusUnimplemented = 58
	// StatusTooManyRequests 表示使用者近期發送太多請求，需稍後再試。
	StatusTooManyRequests = 59
	// StatusResourceExhausted 表示使用者可用的額度已耗盡。
	StatusResourceExhausted = 60
	// StatusBusy 表示伺服器正繁忙無法進行執行請求。
	StatusBusy = 61
	// StatusFileRetry 表示檔案區塊發生錯誤，需要重新上傳相同區塊。
	StatusFileRetry = 70
	// StatusFileEmpty 表示上傳的檔案、區塊是空的。
	StatusFileEmpty = 71
	// StatusFileTooLarge 表示檔案過大無法上傳。
	StatusFileTooLarge = 72
)

var (
	// ErrEventNotFound 表示欲發送的事件沒有被初始化或任何客戶端監聽而無法找到因此發送失敗。
	ErrEventNotFound = errors.New("the event doesn't exist")
	// ErrChannelNotFound 表示欲發送的事件存在，但目標頻道沒有被初始化或任何客戶端監聽而無法找到因此發送失敗。
	ErrChannelNotFound = errors.New("the channel doesn't exist")
)

// H 是常用的資料格式，簡單說就是 `map[string]interface{}` 的別名。
type H map[string]interface{}

// HandlerFunc 是方法處理函式的型態別名。
type HandlerFunc func(*Context)

// New 會建立一個新的 Mego 空白引擎。
func New() *Engine {
	return &Engine{
		Sessions: make(map[string]*Session),
		Events:   make(map[string]*Event),
		Methods:  make(map[string]*Method),
	}
}

// Default 會建立一個帶有 `Recovery` 和 `Logger` 中介軟體的 Mego 引擎。
func Default() *Engine {
	return &Engine{
		Sessions: make(map[string]*Session),
		Events:   make(map[string]*Event),
		Methods:  make(map[string]*Method),
	}
}

// server 是基礎伺服器用來與基本 HTTP 連線進行互動。
type server struct {
	websocket *melody.Melody
}

// ServeHTTP 會將所有的 HTTP 請求轉嫁給 WebSocket。
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.websocket.HandleRequest(w, r)
}

// Engine 是 Mego 最主要的引擎結構體。
type Engine struct {
	// Sessions 儲存了正在連線的所有階段。
	Sessions map[string]*Session
	// Events 儲存了所有可用的事件與其監聽的客戶端資料。
	Events map[string]*Event
	// Methods 是所有可用的方法切片。
	Methods map[string]*Method
	// Option 是這個引擎的設置。
	Option *EngineOption
	// handlers 是保存將會執行的全域中介軟體切片。
	handlers []HandlerFunc
	// noMethod 是當呼叫不存在方式時所會呼叫的處理函式。
	noMethod []HandlerFunc
	//
	websocket *melody.Melody
}

// EngineOption 是引擎的選項設置。
type EngineOption struct {
	// MaxSize 是這個方法允許接收的最大位元組（Bytes）。
	MaxSize int
	// MaxChunkSize 是這個方法允許的區塊最大位元組（Bytes）。
	MaxChunkSize int
	// MaxFileSize 是這個方法允許的檔案最大位元組（Bytes），會在每次接收區塊時結算總計大小，
	// 如果超過此大小則停止接收檔案。
	MaxFileSize int
	// MaxSessions 是引擎能容忍的最大階段連線數量。
	MaxSessions int
	// CheckInterval 是每隔幾秒進行一次階段是否仍存在的連線檢查，
	// 此為輕量檢查而非發送回應至客戶端。
	CheckInterval int
}

// Method 呈現了一個方法。
type Method struct {
	// Name 是此方法的名稱。
	Name string
	// Handlers 是中介軟體與處理函式。
	Handlers []HandlerFunc
	// Processor 是區塊處理介面，如果此方法不允許檔案上傳則忽略此欄位。
	Processor ChunkProcessor
	// Option 是此方法的選項。
	Option *MethodOption
}

// MethodOption 是一個方法的選項。
type MethodOption struct {
	// MaxSize 是這個方法允許接收的最大位元組（Bytes）。此選項會覆蓋引擎設定。
	MaxSize int
	// MaxChunkSize 是這個方法允許的區塊最大位元組（Bytes）。此選項會覆蓋引擎設定。
	MaxChunkSize int
	// MaxFileSize 是這個方法允許的檔案最大位元組（Bytes），會在每次接收區塊時結算總計大小，
	// 如果超過此大小則停止接收檔案。此選項會覆蓋引擎設定。
	MaxFileSize int
}

// Run 會在指定的埠口執行 Mego 引擎。
func (e *Engine) Run(port ...string) {
	// 初始化一個 Melody 套件框架並當作 WebSocket 底層用途。
	m := melody.New()
	// 以 WebSocket 初始化一個底層伺服器。
	s := &server{
		websocket: m,
	}
	e.websocket = m

	// 設定預設埠口。
	p := ":5000"
	if len(port) > 0 {
		p = port[0]
	}

	// 將接收到的所有訊息轉交給控制器。
	m.HandleMessage(e.messageHandler)
	//
	m.HandleConnect(e.connectHandler)

	// 開始在指定埠口監聽 HTTP 請求並交由底層伺服器處理。
	http.ListenAndServe(p, s)
}

// connectHandler 處理連接起始的函式。
func (e *Engine) connectHandler(s *melody.Session) {
	// 替此階段建立一個獨立的 UUID。
	id := uuid.NewV4().String()
	// 在底層階段存放此階段的編號。
	s.Set("ID", id)
	// 將 Mego 階段放入引擎中保存。
	e.Sessions[id] = &Session{
		ID:        id,
		websocket: s,
	}
}

// messageHandler 處理所有接收到的訊息，並轉接給相對應的方法處理函式。
func (e *Engine) messageHandler(s *melody.Session, msg []byte) {
	var req Request

	// 將接收到的資料映射到本地請求建構體。
	if err := msgpack.Unmarshal(msg, &req); err != nil {
		// 如果發生錯誤則建立錯誤回應建構體，並傳送到客戶端。
		resp, _ := msgpack.Marshal(Response{
			Error: ResponseError{
				Code:    StatusInvalid,
				Message: err.Error(),
			},
		})
		s.WriteBinary(resp)
		return
	}

	// 取得這個 WebSocket 階段對應的 Mego 階段。
	id, ok := s.Get("ID")
	if !ok {
		return
	}
	// 透過獨有編號在引擎中找出相對應的階段資料。
	sess, ok := e.Sessions[id.(string)]
	if !ok {
		return
	}

	// 如果這個請求要呼叫的方法是 Mego 的初始化函式。
	if req.Method == "MegoInitialize" {
		// 將接收到的資料映射到本地的 map 型態，並保存到階段資料中的鍵值組。
		var keys map[string]interface{}
		if err := msgpack.Unmarshal(req.Params, &keys); err == nil {
			sess.Keys = keys
		}
		return
	}

	// 如果客戶端離線了就自動移除他所監聽的事件和所有 Sessions

	//
	switch req.Method {
	//
	case "MegoInitialize":
		// 將接收到的資料映射到本地的 map 型態，並保存到階段資料中的鍵值組。
		var keys map[string]interface{}
		if err := msgpack.Unmarshal(req.Params, &keys); err == nil {
			sess.Keys = keys
		}
		return

	//
	case "MegoSubscribe":
		//
	}

	// 呼叫該請求欲呼叫的方法。
	method, ok := e.Methods[req.Method]
	if !ok {
		// 如果該方法不存在，就呼叫不存在方法處理函式。
	}

	// 建立一個上下文建構體。
	ctx := &Context{
		Session:  sess,
		Method:   method,
		ID:       req.ID,
		Request:  s.Request,
		data:     req.Params,
		handlers: e.handlers,
	}
	// 將該方法的處理函式推入上下文建構體中供依序執行。
	ctx.handlers = append(ctx.handlers, method.Handlers...)

	// 如果處理函式數量大於零的話就可以開始執行了。
	if len(ctx.handlers) > 0 {
		ctx.handlers[0](ctx)
	}
}

func (e *Engine) HandleRequest() *Engine {
	return e
}

func (e *Engine) HandleConnect() *Engine {
	return e
}

// HandleSubscribe 會更改預設的事件訂閱檢查函式，開發者可傳入一個回呼函式並接收客戶端欲訂閱的事件與頻道和相關資料。
// 回傳一個 `false` 即表示客戶端的資格不符，將不納入訂閱清單中。該客戶端將無法接收到指定的事件。
func (e *Engine) HandleSubscribe(handler func(event string, channel string, c *Context) bool) *Engine {
	return e
}

// Use 會使用傳入的中介軟體作為全域使用。
func (e *Engine) Use(handlers ...HandlerFunc) *Engine {
	e.handlers = append(e.handlers, handlers...)
	return e
}

// Len 會回傳目前有多少個連線數。
func (e *Engine) Len() int {
	return len(e.Sessions)
}

// Close 會結束此引擎的服務。
func (e *Engine) Close() error {
	e.websocket.Close()
	return nil
}

// NoMethod 會在客戶端呼叫不存在方法時被執行。
func (e *Engine) NoMethod(handler ...HandlerFunc) *Engine {
	e.noMethod = handler
	return e
}

// Event 會建立一個新的事件，如此一來客戶端方能監聽。
func (e *Engine) Event(name string) {
	e.Events[name] = &Event{
		Name: name,
	}
}

// Register 會註冊一個指定的方法，並且允許客戶端呼叫此方法觸發指定韓式。
func (e *Engine) Register(method string, handler ...HandlerFunc) *Method {
	m := &Method{
		Name:     method,
		Handlers: handler,
	}
	e.Methods[method] = m
	return m
}

// Emit 會帶有指定資料並廣播指定事件與頻道，當頻道為空字串時則廣播到所有頻道。
func (e *Engine) Emit(event string, channel string, result interface{}) error {
	ev, ok := e.Events[event]
	if !ok {
		return ErrEventNotFound
	}
	if ch == ""

	ch, ok := ev.Channels[channel]
	if !ok {
		return ErrChannelNotFound
	}
	var firstErr error
	for _, v := range ch.Sessions {
		v.write(Response{
			Event:  event,
			Result: result,
		})
		//err := v.websocket.WriteBinary()
		//if firstErr == nil {
		//	firstErr = err
		//}
	}

	return firstErr
}

// EmitMultiple 會將指定事件與資料向指定的客戶端切片進行廣播。
func (e *Engine) EmitMultiple(event string, result interface{}, sessions []*Session) error {
	return nil
}

// EmitFilter 會以過濾函式來決定要將帶有指定資料的事件廣播給誰。
// 如果過濾函式回傳 `true` 則表示該客戶端會接收到該事件。
func (e *Engine) EmitFilter(event string, payload interface{}, filter func(*Session) bool) error {
	return nil
}

// Receive 會建立一個指定的方法，並且允許客戶端傳送檔案至此方法。
func (e *Engine) Receive(method string, handler ...HandlerFunc) *Method {
	return e.ReceiveWith(method, &DefaultChunkProcessor{}, handler...)
}

// ReceiveWith 會透過自訂的區塊處理函式建立指定方法，讓客戶端可上傳檔案至此方法並透過自訂方式進行處理。
func (e *Engine) ReceiveWith(method string, processor ChunkProcessor, handler ...HandlerFunc) *Method {
	m := e.Register(method, handler...)
	m.Processor = processor
	return m
}
