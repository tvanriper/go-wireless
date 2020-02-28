# go-wireless

A way to interact with the Wireless interfaces on a Linux machine using WPA Supplicant.

## Requirements

Requires a running wpa_supplicant with control interface at `/var/run/wpa_supplicant`.

# Usage

Get a list of wifi cards attached:

```
ifaces := wireless.Interfaces()
```

From there you can use the client:

```
wc, err := wireless.NewClient("wlan0")
defer wc.Close()

// get a list of APs that are in range
aps, err := wc.Scan()
fmt.Println(aps, err)

// get a list of known networks
nets, err := wc.Networks()
fmt.Println(nets, err)
```

Subsscibe to events:

```
sub := wc.Subscribe(wireless.EventConnected, wireless.EventAuthReject, wireless.EventDisconnected)

ev := <-sub.Next
switch ev.Name {
	case wireless.EventConnected:
		fmt.Println(ev.Arguments)
	case wireless.EventAuthReject:
		fmt.Println(ev.Arguments)
	case wireless.EventDisconnected:
		fmt.Println(ev.Arguments)
}
```