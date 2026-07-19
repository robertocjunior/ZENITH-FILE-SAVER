package whatsapp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// ClientManager wraps whatsmeow.Client and handles connection, pairing, and events
type ClientManager struct {
	client      *whatsmeow.Client
	dbContainer *sqlstore.Container
	dbPath      string

	mu           sync.RWMutex
	state        string // "disconnected", "connecting", "connected"
	qrCode       string // Base64 image
	monitoredJID string

	// Callbacks for events
	logCallback   func(string, ...interface{})
	qrCallback    func(string)
	stateCallback func(string)
	mediaCallback func(data []byte, mimeType, originalName, senderName, senderJID string, timestamp time.Time)
}

// NewClientManager initializes a ClientManager
func NewClientManager(
	dbPath string,
	logCallback func(string, ...interface{}),
	qrCallback func(string),
	stateCallback func(string),
	mediaCallback func([]byte, string, string, string, string, time.Time),
) *ClientManager {
	return &ClientManager{
		dbPath:        dbPath,
		state:         "disconnected",
		logCallback:   logCallback,
		qrCallback:    qrCallback,
		stateCallback: stateCallback,
		mediaCallback: mediaCallback,
	}
}

// Start opens the database container and attempts to connect/reconnect
func (m *ClientManager) Start() error {
	m.log("Initializing WhatsApp database session container...")
	container, err := sqlstore.New(context.Background(), "sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", m.dbPath), waLog.Noop)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database for whatsmeow: %w", err)
	}
	m.dbContainer = container

	deviceStore, err := m.dbContainer.GetFirstDevice(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get first device from store: %w", err)
	}

	m.log("Creating whatsmeow client...")
	m.client = whatsmeow.NewClient(deviceStore, waLog.Noop)
	m.client.AddEventHandler(m.handleEvent)

	return m.Connect()
}

// Connect establishes connection to WhatsApp servers
func (m *ClientManager) Connect() error {
	m.mu.Lock()
	if m.state == "connected" || m.state == "connecting" {
		m.mu.Unlock()
		return nil
	}
	m.state = "connecting"
	m.stateCallback(m.state)
	m.mu.Unlock()

	m.log("Connecting to WhatsApp...")
	
	if m.client.Store.ID == nil {
		// New login, start QR flow
		go m.qrLoginFlow()
	} else {
		// Existing login, connect directly
		err := m.client.Connect()
		if err != nil {
			m.updateState("disconnected")
			return fmt.Errorf("failed to connect: %w", err)
		}
		m.updateState("connected")
	}

	return nil
}

// SetMonitoredGroup JID string
func (m *ClientManager) SetMonitoredGroup(jid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.monitoredJID = jid
	m.log("Monitored group set to: %s", jid)
}

// GetGroups returns the list of groups the user has joined
func (m *ClientManager) GetGroups() ([]*types.GroupInfo, error) {
	m.mu.RLock()
	client := m.client
	state := m.state
	m.mu.RUnlock()

	if state != "connected" || client == nil {
		return nil, errors.New("WhatsApp is not connected")
	}

	groups, err := client.GetJoinedGroups(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch joined groups: %w", err)
	}

	return groups, nil
}

// Disconnect closes the WhatsApp connection
func (m *ClientManager) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client != nil {
		m.log("Disconnecting from WhatsApp...")
		m.client.Disconnect()
	}
	m.state = "disconnected"
	m.stateCallback(m.state)
}

// Logout removes the session store and disconnects
func (m *ClientManager) Logout() error {
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()

	if client == nil {
		return errors.New("client is not initialized")
	}

	m.log("Logging out of WhatsApp and clearing session...")
	err := client.Logout(context.Background())
	if err != nil {
		// If logout fails, we still want to force disconnect
		m.Disconnect()
		return fmt.Errorf("failed to log out: %w", err)
	}

	m.updateState("disconnected")
	return nil
}

// GetState returns the current state and QR code base64
func (m *ClientManager) GetState() (string, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state, m.qrCode
}

// qrLoginFlow handles the QR generation and pairing flow
func (m *ClientManager) qrLoginFlow() {
	qrChan, err := m.client.GetQRChannel(context.Background())
	if err != nil {
		m.log("Failed to get QR channel: %v", err)
		m.updateState("disconnected")
		return
	}

	err = m.client.Connect()
	if err != nil {
		m.log("Failed to connect for login: %v", err)
		m.updateState("disconnected")
		return
	}

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			m.log("New QR code received, rendering...")
			pngBytes, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
			if err != nil {
				m.log("Failed to encode QR code to PNG: %v", err)
				continue
			}

			qrBase64 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
			m.mu.Lock()
			m.qrCode = qrBase64
			m.mu.Unlock()
			m.qrCallback(qrBase64)

		case "success":
			m.log("Successfully logged in via QR Code!")
			m.mu.Lock()
			m.qrCode = ""
			m.mu.Unlock()
			m.qrCallback("")
			m.updateState("connected")

		case "timeout":
			m.log("QR pairing timed out.")
			m.mu.Lock()
			m.qrCode = ""
			m.mu.Unlock()
			m.qrCallback("")
			m.updateState("disconnected")
		}
	}
}

// handleEvent handles incoming events from whatsmeow client
func (m *ClientManager) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Connected:
		m.updateState("connected")
		m.log("WhatsApp server connected.")

	case *events.LoggedOut:
		m.updateState("disconnected")
		m.log("Logged out from WhatsApp.")

	case *events.Disconnected:
		// If disconnected, whatsmeow automatically reconnects in the background.
		// We only set to "connecting" if it was connected before.
		m.mu.Lock()
		if m.state == "connected" {
			m.state = "connecting"
			m.stateCallback(m.state)
		}
		m.mu.Unlock()
		m.log("Disconnected from WhatsApp. Reconnecting...")

	case *events.Message:
		m.handleMessage(v)
	}
}

// handleMessage filters message events for media inside the monitored group
func (m *ClientManager) handleMessage(evt *events.Message) {
	m.mu.RLock()
	targetJID := m.monitoredJID
	m.mu.RUnlock()

	// If no group is monitored, ignore
	if targetJID == "" {
		return
	}

	// Verify if the message is from the monitored group JID
	// The message chat JID string is checked against our monitored group JID.
	if evt.Info.Chat.String() != targetJID {
		return
	}

	// We support files sent by other users AND self-sent messages (from our own account).
	// No filter on Info.IsFromMe is applied here, which satisfies the user's explicit request.

	// Determine if there is media attachment in the message
	var downloadable whatsmeow.DownloadableMessage
	var mimeType string
	var originalName string

	msg := evt.Message
	if msg == nil {
		return
	}

	if img := msg.GetImageMessage(); img != nil {
		downloadable = img
		mimeType = img.GetMimetype()
		originalName = "imagem.jpg" // Default fallback name for image message
	} else if doc := msg.GetDocumentMessage(); doc != nil {
		downloadable = doc
		mimeType = doc.GetMimetype()
		originalName = doc.GetFileName()
		if originalName == "" {
			originalName = "documento"
		}
	} else if video := msg.GetVideoMessage(); video != nil {
		downloadable = video
		mimeType = video.GetMimetype()
		originalName = "video.mp4"
	} else if audio := msg.GetAudioMessage(); audio != nil {
		downloadable = audio
		mimeType = audio.GetMimetype()
		originalName = "audio.ogg"
	}

	// If a downloadable media attachment was found, process it
	if downloadable != nil {
		senderName := evt.Info.PushName
		if senderName == "" {
			// Fallback to phone number if push name is not available
			senderName = evt.Info.Sender.User
		}

		m.log("Received media attachment from '%s'. Downloading...", senderName)

		// Create a context with timeout for media download
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		data, err := m.client.Download(ctx, downloadable)
		if err != nil {
			m.log("Error downloading media from '%s': %v", senderName, err)
			return
		}

		m.log("Media downloaded successfully (%d bytes). Handing over for processing...", len(data))
		// Call the media callback asynchronously to not block whatsmeow's event loop
		go m.mediaCallback(data, mimeType, originalName, senderName, evt.Info.Sender.String(), evt.Info.Timestamp)
	}
}

// updateState helper to change the state and trigger callbacks
func (m *ClientManager) updateState(newState string) {
	m.mu.Lock()
	m.state = newState
	if newState != "disconnected" {
		m.qrCode = ""
	}
	m.mu.Unlock()

	m.stateCallback(newState)
}

// log helper to send logs to logCallback
func (m *ClientManager) log(format string, args ...interface{}) {
	if m.logCallback != nil {
		m.logCallback(format, args...)
	}
}
