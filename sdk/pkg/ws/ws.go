package ws

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/go-admin-team/go-admin-core/v2/sdk/pkg"
)

// Manager 所有 websocket 信息
type Manager struct {
	Group                   map[string]map[string]*Client
	groupCount, clientCount uint
	Lock                    sync.Mutex
	Register, UnRegister    chan *Client
	Message                 chan *MessageData
	GroupMessage            chan *GroupMessageData
	BroadCastMessage        chan *BroadCastMessageData
}

// Client 单个 websocket 信息
type Client struct {
	Id, Group  string
	Context    context.Context
	CancelFunc context.CancelFunc
	Socket     *websocket.Conn
	Message    chan []byte
}

// MessageData 单个发送数据信息
type MessageData struct {
	Id, Group string
	Context   context.Context
	Message   []byte
}

// GroupMessageData 组广播数据信息
type GroupMessageData struct {
	Group   string
	Message []byte
}

// BroadCastMessageData 广播发送数据信息
type BroadCastMessageData struct {
	Message []byte
}

// Read 读信息，从 websocket 连接直接读取数据
func (c *Client) Read(cxt context.Context) {
	defer func(cxt context.Context) {
		WebsocketManager.UnRegister <- c
		log.Printf("client [%s] disconnect", c.Id)
		if err := c.Socket.Close(); err != nil {
			log.Printf("client [%s] disconnect err: %s", c.Id, err)
		}
	}(cxt)

	for {
		if cxt.Err() != nil {
			break
		}
		messageType, message, err := c.Socket.ReadMessage()
		if err != nil || messageType == websocket.CloseMessage {
			break
		}
		log.Printf("client [%s] receive message: %s", c.Id, string(message))
		c.Message <- message
	}
}

// Write 写信息，从 channel 变量 Send 中读取数据写入 websocket 连接
func (c *Client) Write(cxt context.Context) {
	defer func(cxt context.Context) {
		log.Printf("client [%s] disconnect", c.Id)
		if err := c.Socket.Close(); err != nil {
			log.Printf("client [%s] disconnect err: %s", c.Id, err)
		}
	}(cxt)

	for {
		if cxt.Err() != nil {
			break
		}
		select {
		case message, ok := <-c.Message:
			if !ok {
				_ = c.Socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			log.Printf("client [%s] write message: %s", c.Id, string(message))
			err := c.Socket.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("client [%s] writemessage err: %s", c.Id, err)
			}
		case <-c.Context.Done():
			// break here leaves the select, not the loop. What ended the
			// loop was the check at the top, which watches the parameter —
			// so this exit worked only while the caller passed the client's
			// own context, which the signature does not require.
			return
		}
	}
}

// Start 启动 websocket 管理器
func (manager *Manager) Start() {
	log.Printf("websocket manage start")
	for {
		select {
		// 注册
		case client := <-manager.Register:
			log.Printf("client [%s] connect", client.Id)
			log.Printf("register client [%s] to group [%s]", client.Id, client.Group)

			manager.Lock.Lock()
			if manager.Group[client.Group] == nil {
				manager.Group[client.Group] = make(map[string]*Client)
				manager.groupCount += 1
			}
			manager.Group[client.Group][client.Id] = client
			manager.clientCount += 1
			manager.Lock.Unlock()

		// 注销
		case client := <-manager.UnRegister:
			log.Printf("unregister client [%s] from group [%s]", client.Id, client.Group)
			manager.Lock.Lock()
			if mGroup, ok := manager.Group[client.Group]; ok {
				if mClient, ok := mGroup[client.Id]; ok {
					close(mClient.Message)
					delete(mGroup, client.Id)
					manager.clientCount -= 1
					if len(mGroup) == 0 {
						//log.Printf("delete empty group [%s]", client.Group)
						delete(manager.Group, client.Group)
						manager.groupCount -= 1
					}
					mClient.CancelFunc()
				}
			}
			manager.Lock.Unlock()

			// 发送广播数据到某个组的 channel 变量 Send 中
			//case data := <-manager.boardCast:
			//	if groupMap, ok := manager.wsGroup[data.GroupId]; ok {
			//		for _, conn := range groupMap {
			//			conn.Send <- data.Data
			//		}
			//	}
		}
	}
}

// SendService 处理单个 client 发送数据
func (manager *Manager) SendService() {
	for {
		select {
		case data := <-manager.Message:
			if groupMap, ok := manager.Group[data.Group]; ok {
				if conn, ok := groupMap[data.Id]; ok {
					conn.Message <- data.Message
				}
			}
		}
	}
}

// SendGroupService 处理 group 广播数据
func (manager *Manager) SendGroupService() {
	for {
		select {
		// 发送广播数据到某个组的 channel 变量 Send 中
		case data := <-manager.GroupMessage:
			if groupMap, ok := manager.Group[data.Group]; ok {
				for _, conn := range groupMap {
					conn.Message <- data.Message
				}
			}
		}
	}
}

// SendAllService 处理广播数据
func (manager *Manager) SendAllService() {
	for {
		select {
		case data := <-manager.BroadCastMessage:
			for _, v := range manager.Group {
				for _, conn := range v {
					conn.Message <- data.Message
				}
			}
		}
	}
}

// Send 向指定的 client 发送数据
func (manager *Manager) Send(cxt context.Context, id string, group string, message []byte) {
	data := &MessageData{
		Id:      id,
		Context: cxt,
		Group:   group,
		Message: message,
	}
	manager.Message <- data
}

// SendGroup 向指定的 Group 广播
func (manager *Manager) SendGroup(group string, message []byte) {
	data := &GroupMessageData{
		Group:   group,
		Message: message,
	}
	manager.GroupMessage <- data
}

// SendAll 广播
func (manager *Manager) SendAll(message []byte) {
	data := &BroadCastMessageData{
		Message: message,
	}
	manager.BroadCastMessage <- data
}

// RegisterClient 注册
func (manager *Manager) RegisterClient(client *Client) {
	manager.Register <- client
}

// UnRegisterClient 注销
func (manager *Manager) UnRegisterClient(client *Client) {
	manager.UnRegister <- client
}

// LenGroup 当前组个数
func (manager *Manager) LenGroup() uint {
	return manager.groupCount
}

// LenClient 当前连接个数
func (manager *Manager) LenClient() uint {
	return manager.clientCount
}

// Info 获取 wsManager 管理器信息
func (manager *Manager) Info() map[string]interface{} {
	managerInfo := make(map[string]interface{})
	managerInfo["groupLen"] = manager.LenGroup()
	managerInfo["clientLen"] = manager.LenClient()
	managerInfo["chanRegisterLen"] = len(manager.Register)
	managerInfo["chanUnregisterLen"] = len(manager.UnRegister)
	managerInfo["chanMessageLen"] = len(manager.Message)
	managerInfo["chanGroupMessageLen"] = len(manager.GroupMessage)
	managerInfo["chanBroadCastMessageLen"] = len(manager.BroadCastMessage)
	return managerInfo
}

// WebsocketManager 初始化 wsManager 管理器
var WebsocketManager = Manager{
	Group:            make(map[string]map[string]*Client),
	Register:         make(chan *Client, 128),
	UnRegister:       make(chan *Client, 128),
	GroupMessage:     make(chan *GroupMessageData, 128),
	Message:          make(chan *MessageData, 128),
	BroadCastMessage: make(chan *BroadCastMessageData, 128),
	groupCount:       0,
	clientCount:      0,
}

// WsClient gin 处理 websocket handler
func (manager *Manager) WsClient(c *gin.Context) {

	upGrader := websocket.Upgrader{
		// cross origin domain
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols: []string{c.GetHeader("Sec-WebSocket-Protocol")},
	}

	conn, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket connect error: %s", c.Param("channel"))
		return
	}

	// Created after a successful handshake: on the failure path above the
	// cancel func would never run, leaking the context.
	ctx, cancel := context.WithCancel(context.Background())

	fmt.Println("token: ", c.Query("token"))

	client := &Client{
		Id:         c.Param("id"),
		Group:      c.Param("channel"),
		Context:    ctx,
		CancelFunc: cancel,
		Socket:     conn,
		Message:    make(chan []byte, 1024),
	}

	manager.RegisterClient(client)
	go client.Read(ctx)
	go client.Write(ctx)
	time.Sleep(time.Second * 15)

	pkg.FileMonitoringById(ctx, "temp/logs/job/db-20200820.log", c.Param("id"), c.Param("channel"), SendOne)
}

// UnWsClient gin 处理 websocket 关闭
func (manager *Manager) UnWsClient(c *gin.Context) {
	id := c.Param("id")
	group := c.Param("channel")
	WsLogout(id, group)
	c.Set("result", "ws close success")
	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": "ws close success",
		"msg":  "success",
	})
}

// SendGroup 向指定的 Group 广播
func SendGroup(group string, msg []byte) {
	WebsocketManager.SendGroup(group, []byte("{\"code\":200,\"data\":"+string(msg)+"}"))
	fmt.Println(WebsocketManager.Info())
}

// SendAll 广播
func SendAll(msg []byte) {
	WebsocketManager.SendAll([]byte("{\"code\":200,\"data\":" + string(msg) + "}"))
	fmt.Println(WebsocketManager.Info())
}

// SendOne 向指定的 client 发送数据
func SendOne(ctx context.Context, id string, group string, msg []byte) {
	WebsocketManager.Send(ctx, id, group, []byte("{\"code\":200,\"data\":"+string(msg)+"}"))
	fmt.Println(WebsocketManager.Info())
}

// WsLogout 注销
func WsLogout(id string, group string) {
	WebsocketManager.UnRegisterClient(&Client{Id: id, Group: group})
	fmt.Println(WebsocketManager.Info())
}
