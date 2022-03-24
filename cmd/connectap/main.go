package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/tvanriper/go-wireless"
)

func main() {
	var iface, ssid, pks string
	defIface, ok := wireless.DefaultInterface()
	if !ok {
		panic("no wifi cards on the system")
	}
	flag.StringVar(&iface, "i", defIface, "interface to use")
	flag.StringVar(&ssid, "s", "", "ssid to use")
	flag.StringVar(&pks, "p", "", "password to use")
	flag.Parse()

	if len(ssid) == 0 {
		ok = false
		fmt.Println("must provide a ssid (-s)")
	}

	if !ok {
		os.Exit(1)
	}

	fmt.Printf("Using interface: %s\n", iface)

	wc, err := wireless.NewClient(iface)
	if err != nil {
		fmt.Printf("unable to reach %s: %s\n", iface, err)
		os.Exit(1)
	}
	defer wc.Close()

	state, err := wc.Status()
	if err != nil {
		fmt.Printf("problems getting the current status: %s\n", err)
		os.Exit(1)
	}

	net := wireless.NewNetwork(ssid, pks)

	if len(state.IPAddress) > 0 {
		// Must disconnect first.
		fmt.Printf("Disconnecting from %s.\n", state.SSID)
		_, err = wc.Disconnect(wireless.NewNetwork(state.SSID, ""))
		if err != nil {
			fmt.Printf("failed to disconnect network: %s\n", err)
			os.Exit(1)
		}
		// Pause for a bit to let messages run.
		time.Sleep(time.Second)
	}

	fmt.Printf("Attempting to connect to %s...\n", ssid)
	_, err = wc.Connect(net)
	if err != nil {
		fmt.Printf("did not connect to %s:\n", ssid)
		switch err {
		case wireless.ErrSSIDNotFound:
			fmt.Println("SSID not found")
		case wireless.ErrAuthFailed:
			fmt.Println("Bad password")
		case wireless.ErrDisconnected:
			fmt.Println("Disconnected")
		case wireless.ErrAssocRejected:
			fmt.Println("Assoc rejected")
		default:
			fmt.Println("failed to save configuration")
		}
		fmt.Println("Use wifistate to monitor for changes.")
		os.Exit(1)
	}

	fmt.Println("Connected to " + net.SSID)
}
