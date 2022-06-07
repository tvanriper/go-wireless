package wireless

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// WPAConn is an interface to the connection
type WPAConn interface {
	SendCommand(...string) (string, error)
	SendCommandBool(...string) error
	SendCommandInt(...string) (int, error)
	io.Closer
	Subscribe(...string) *Subscription
}

// Client represents a wireless client
type Client struct {
	conn WPAConn
	mu   sync.Mutex
}

// NewClient will create a new client by connecting to the
// given interface in WPA
func NewClient(iface string) (c *Client, err error) {
	c = new(Client)
	c.conn, err = Dial(iface)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// NewClientFromConn returns a new client from an already established connection
func NewClientFromConn(conn WPAConn) (c *Client) {
	c.conn = conn
	return c
}

// Close will close the client connection
func (cl *Client) Close() {
	cl.conn.Close()
}

// Conn will return the underlying connection
func (cl *Client) Conn() *Conn {
	return cl.conn.(*Conn)
}

// Subscribe will subscribe to certain events that happen in WPA
func (cl *Client) Subscribe(topics ...string) *Subscription {
	return cl.conn.Subscribe(topics...)
}

// Status will return the current state of the WPA
func (cl *Client) Status() (State, error) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	data, err := cl.conn.SendCommand(CmdStatus)
	if err != nil {
		return State{}, err
	}
	s := NewState(data)
	return s, nil
}

// Scan will scan for networks and return the APs it finds
func (cl *Client) Scan() (nets APs, err error) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	err = cl.conn.SendCommandBool(CmdScan)
	if err != nil {
		return nets, err
	}

	results := cl.conn.Subscribe(EventScanResults)
	defer results.Unsubscribe()
	failed := cl.conn.Subscribe(EventScanFailed)
	defer failed.Unsubscribe()
	timer := time.NewTimer(time.Second * 2).C

	func() {
		for {
			select {
			case <-failed.Next():
				err = ErrScanFailed
				return
			case <-results.Next():
				return
			case <-timer:
				return
			}
		}
	}()

	scanned, err := cl.conn.SendCommand(CmdScanResults)
	if err != nil {
		return nets, err
	}

	return parseAP([]byte(scanned))
}

// Networks lists the known networks
func (cl *Client) Networks() (nets Networks, err error) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	data, err := cl.conn.SendCommand(CmdListNetworks)
	if err != nil {
		return nil, err
	}

	nets, err = parseNetwork([]byte(data))
	if err != nil {
		return nil, err
	}

	for i := range nets {
		nets[i].Known = true
		(&nets[i]).populateAttrs(cl)
	}

	return nets, nil
}

// Connect to a new or existing network
func (cl *Client) Connect(net Network) (Network, error) {
	net, err := cl.AddOrUpdateNetwork(net)
	if err != nil {
		return net, err
	}

	sub := cl.conn.Subscribe(EventNetworkNotFound, EventAuthReject, EventConnected, EventDisconnected, EventAssocReject)
	defer sub.Unsubscribe()
	if net.IsDisabled() {
		fmt.Printf("Enabling network %d\n", net.ID)
		if err := cl.EnableNetwork(net.ID); err != nil {
			return net, err
		}
	} else {
		fmt.Printf("Selecting network %d\n", net.ID)
		if err := cl.SelectNetwork(net.ID); err != nil {
			return net, err
		}
	}

	ev := <-sub.Next()

	switch ev.Name {
	case EventConnected:
		return net, cl.SaveConfig()
	case EventNetworkNotFound:
		return net, ErrSSIDNotFound
	case EventAuthReject:
		return net, ErrAuthFailed
	case EventDisconnected:
		return net, ErrDisconnected
	case EventAssocReject:
		return net, ErrAssocRejected
	}

	return net, errors.New("failed to catch event " + ev.Name)
}

func (cl *Client) Disconnect() (err error) {
	_, err = cl.conn.SendCommand(CmdDisconnect)
	return err
}

// AddOrUpdateNetwork will add or, if the network has IDStr set, update it
func (cl *Client) AddOrUpdateNetwork(net Network) (Network, error) {
	if len(net.IDStr) == 0 {
		nets, err := cl.Networks()
		if err != nil {
			return net, err
		}

		_, ok := nets.FindByIDStr(net.IDStr)
		if ok {
			return cl.UpdateNetwork(net)
		}
	}

	// I don't think it's right for the same SSID to have
	// multiple passwords.  Check for a matching SSID.
	if len(net.SSID) != 0 {
		nets, err := cl.Networks()
		if err != nil {
			return net, err
		}

		found, ok := nets.FindBySSID(net.SSID)
		if ok {
			net.ID = found.ID
			net.IDStr = found.IDStr
			return cl.UpdateNetwork(net)
		}
	}

	return cl.AddNetwork(net)
}

// UpdateNetwork will update the given network, an error will be thrown
// if the network doesn't have IDStr specified
func (cl *Client) UpdateNetwork(net Network) (Network, error) {
	if net.IDStr == "" {
		return net, ErrNoIdentifier
	}

	for _, cmd := range setCmds(net) {
		if err := cl.conn.SendCommandBool(cmd); err != nil {
			return net, errors.Wrap(err, cmd)
		}
	}

	return net, nil
}

// AddNetwork will add a new network
func (cl *Client) AddNetwork(net Network) (Network, error) {
	i, err := cl.conn.SendCommandInt(CmdAddNetwork)
	if err != nil {
		return net, err
	}

	net.ID = i

	if net.IDStr == "" {
		net.IDStr = net.SSID
	}

	for _, cmd := range setCmds(net) {
		if err := cl.conn.SendCommandBool(cmd); err != nil {
			return net, err
		}
	}

	net.Known = true
	return net, nil
}

// RemoveNetwork will RemoveNetwork
func (cl *Client) RemoveNetwork(id int) error {
	return cl.conn.SendCommandBool(CmdRemoveNetwork, strconv.Itoa(id))
}

// EnableNetwork will EnableNetwork
func (cl *Client) EnableNetwork(id int) error {
	return cl.conn.SendCommandBool(CmdEnableNetwork + " " + strconv.Itoa(id))
}

func (cl *Client) SelectNetwork(id int) error {
	return cl.conn.SendCommandBool(CmdSelectNetwork + " " + strconv.Itoa(id))
}

// DisableNetwork will DisableNetwork
func (cl *Client) DisableNetwork(id int) error {
	return cl.conn.SendCommandBool(CmdDisableNetwork + " " + strconv.Itoa(id))
}

// SaveConfig will SaveConfig
func (cl *Client) SaveConfig() error {
	return cl.conn.SendCommandBool(CmdSaveConfig)
}

// LoadConfig will LoadConfig
func (cl *Client) LoadConfig() error {
	return cl.conn.SendCommandBool(CmdReconfigure)
}

// GetNetworkAttr will get the given attribute of the given network
func (cl *Client) GetNetworkAttr(id int, attr string) (string, error) {
	s, err := cl.conn.SendCommand(CmdGetNetwork, strconv.Itoa(id), attr)
	if err != nil {
		return s, err
	}

	return strings.TrimSpace(s), nil
}
