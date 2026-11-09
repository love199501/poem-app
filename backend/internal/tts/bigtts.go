package tts

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// 豆包大模型语音（seed-tts-2.0）WebSocket 双向协议适配。
// 仅需 X-Api-Key，无需 AppID。参考字节官方 bidirection 示例。

const (
	bigURL = "wss://openspeech.bytedance.com/api/v3/tts/bidirection"

	// 消息类型（byte1 高4位）
	mtFullClient      = 0b0001
	mtFullServer      = 0b1001
	mtAudioOnlyServer = 0b1011
	mtError           = 0b1111

	// flag（byte1 低4位）
	flagWithEvent = 0b0100

	// 序列化/压缩（byte2）
	serJSON  = 0b0001
	compNone = 0b0000

	// 事件
	evStartConnection    = 1
	evFinishConnection   = 2
	evConnectionStarted  = 50
	evConnectionFailed   = 51
	evConnectionFinished = 52
	evStartSession       = 100
	evFinishSession      = 102
	evSessionStarted     = 150
	evSessionFinished    = 152
	evSessionFailed      = 153
	evTaskRequest        = 200
)

type bigMessage struct {
	Type      int
	Event     int32
	SessionID string
	ErrorCode uint32
	Payload   []byte
}

// eventHasSession 连接级事件不带 sessionID，会话级事件才带。
func eventHasSession(ev int32) bool {
	switch ev {
	case evStartConnection, evFinishConnection, evConnectionStarted, evConnectionFailed, evConnectionFinished:
		return false
	}
	return true
}

// encode 客户端帧：带 event；仅会话级事件带 sessionID；再带 payload。
func encodeBig(msgType int, event int32, sessionID string, payload []byte) []byte {
	var b bytes.Buffer
	b.WriteByte(0x11) // version=1, headerSize=1(4字节)
	b.WriteByte(byte(msgType<<4) | flagWithEvent)
	b.WriteByte(byte(serJSON<<4) | compNone)
	b.WriteByte(0x00)
	_ = binary.Write(&b, binary.BigEndian, event)
	if eventHasSession(event) {
		sid := []byte(sessionID)
		_ = binary.Write(&b, binary.BigEndian, int32(len(sid)))
		b.Write(sid)
	}
	_ = binary.Write(&b, binary.BigEndian, int32(len(payload)))
	b.Write(payload)
	return b.Bytes()
}

// decode 服务端帧。
func decodeBig(frame []byte) (bigMessage, error) {
	var m bigMessage
	if len(frame) < 4 {
		return m, fmt.Errorf("帧过短")
	}
	headerSize := int(frame[0]&0x0f) * 4
	m.Type = int(frame[1] >> 4)
	flag := int(frame[1] & 0x0f)
	off := headerSize
	rd := func(n int) ([]byte, error) {
		if off+n > len(frame) {
			return nil, fmt.Errorf("帧解析越界")
		}
		s := frame[off : off+n]
		off += n
		return s, nil
	}
	if flag&flagWithEvent != 0 {
		e, err := rd(4)
		if err != nil {
			return m, err
		}
		m.Event = int32(binary.BigEndian.Uint32(e))
		if eventHasSession(m.Event) {
			l, err := rd(4)
			if err != nil {
				return m, err
			}
			sid, err := rd(int(binary.BigEndian.Uint32(l)))
			if err != nil {
				return m, err
			}
			m.SessionID = string(sid)
		}
	}
	if m.Type == mtError {
		e, err := rd(4)
		if err != nil {
			return m, err
		}
		m.ErrorCode = binary.BigEndian.Uint32(e)
	}
	l, err := rd(4)
	if err != nil {
		return m, err
	}
	p, err := rd(int(binary.BigEndian.Uint32(l)))
	if err != nil {
		return m, err
	}
	m.Payload = p
	return m, nil
}

// VolcanoBigClient 豆包大模型语音客户端。
type VolcanoBigClient struct {
	Key        string
	ResourceID string
	Speaker    string
	SpeechRate int
	Emotion    string
}

func NewVolcanoBig(key, resourceID, speaker string, speechRate int, emotion string) *VolcanoBigClient {
	if resourceID == "" {
		resourceID = "seed-tts-2.0"
	}
	if speaker == "" {
		speaker = "zh_female_gaolengyujie_uranus_bigtts"
	}
	return &VolcanoBigClient{Key: key, ResourceID: resourceID, Speaker: speaker, SpeechRate: speechRate, Emotion: emotion}
}

func (c *VolcanoBigClient) Configured() bool { return c.Key != "" }

func connID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (c *VolcanoBigClient) send(ws *websocket.Conn, frame []byte) error {
	_ = ws.SetWriteDeadline(time.Now().Add(15 * time.Second))
	return ws.WriteMessage(websocket.BinaryMessage, frame)
}

func (c *VolcanoBigClient) read(ws *websocket.Conn) (bigMessage, error) {
	_ = ws.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		return bigMessage{}, err
	}
	return decodeBig(data)
}

func (c *VolcanoBigClient) waitEvent(ws *websocket.Conn, want int32) error {
	for {
		m, err := c.read(ws)
		if err != nil {
			return err
		}
		if m.Type == mtError {
			return fmt.Errorf("错误(%d): %s", m.ErrorCode, string(m.Payload))
		}
		if m.Type == mtFullServer {
			if m.Event == want {
				return nil
			}
			if m.Event == evConnectionFailed || m.Event == evSessionFailed {
				return fmt.Errorf("失败(event %d): %s", m.Event, string(m.Payload))
			}
		}
	}
}

func (c *VolcanoBigClient) params(text string) map[string]any {
	audio := map[string]any{
		"format":      "mp3",
		"sample_rate": 24000,
		"speech_rate": c.SpeechRate, // 负值更慢，朗诵更从容
	}
	rp := map[string]any{
		"speaker":      c.Speaker,
		"audio_params": audio,
	}
	if c.Emotion != "" {
		rp["emotion"] = c.Emotion
		rp["enable_emotion"] = true
	}
	if text != "" {
		rp["text"] = text
	}
	return rp
}

// Synthesize 走 WS 双向协议合成 mp3。instruction 在此不使用。
func (c *VolcanoBigClient) Synthesize(text, _ string) ([]byte, error) {
	headers := http.Header{}
	headers.Set("X-Api-Key", c.Key)
	headers.Set("X-Api-Resource-Id", c.ResourceID)
	headers.Set("X-Api-Connect-Id", connID())

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	ws, _, err := dialer.Dial(bigURL, headers)
	if err != nil {
		return nil, fmt.Errorf("连接豆包语音失败: %v", err)
	}
	defer ws.Close()

	if err := c.send(ws, encodeBig(mtFullClient, evStartConnection, "", []byte("{}"))); err != nil {
		return nil, err
	}
	if err := c.waitEvent(ws, evConnectionStarted); err != nil {
		return nil, err
	}

	sid := connID()
	startPayload, _ := json.Marshal(map[string]any{"event": evStartSession, "req_params": c.params("")})
	if err := c.send(ws, encodeBig(mtFullClient, evStartSession, sid, startPayload)); err != nil {
		return nil, err
	}
	if err := c.waitEvent(ws, evSessionStarted); err != nil {
		return nil, err
	}

	taskPayload, _ := json.Marshal(map[string]any{"event": evTaskRequest, "req_params": c.params(text)})
	if err := c.send(ws, encodeBig(mtFullClient, evTaskRequest, sid, taskPayload)); err != nil {
		return nil, err
	}
	_ = c.send(ws, encodeBig(mtFullClient, evFinishSession, sid, []byte("{}")))

	var audio bytes.Buffer
recv:
	for {
		m, err := c.read(ws)
		if err != nil {
			return nil, err
		}
		switch m.Type {
		case mtAudioOnlyServer:
			audio.Write(m.Payload)
		case mtError:
			return nil, fmt.Errorf("豆包TTS错误(%d): %s", m.ErrorCode, string(m.Payload))
		case mtFullServer:
			if m.Event == evSessionFailed {
				return nil, fmt.Errorf("豆包TTS会话失败: %s", string(m.Payload))
			}
			if m.Event == evSessionFinished {
				break recv
			}
		}
	}

	_ = c.send(ws, encodeBig(mtFullClient, evFinishConnection, "", []byte("{}")))
	if audio.Len() == 0 {
		return nil, fmt.Errorf("豆包TTS未返回音频")
	}
	return audio.Bytes(), nil
}
