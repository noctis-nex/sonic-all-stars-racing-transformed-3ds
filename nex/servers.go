// Package nex wires up the two PRUDP servers Sonic & All-Stars Racing Transformed
// talks to.
//
// The server uses the NEX libraries and handlers listed in README.md.
// Game-specific wire compatibility has not been fully validated. Unknown
// protocol IDs are logged so they can be investigated during development.
package nex

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	nex "github.com/PretendoNetwork/nex-go/v2"

	"github.com/Protarium-Network/sonic-all-stars-racing-transformed-3ds/globals"
)

// AccessKey is read from PN_SONIC3DS_ACCESS_KEY so the game-specific value
// does not need to be committed to the public source repository.
func accessKey() string { return os.Getenv("PN_SONIC3DS_ACCESS_KEY") }

var (
	registeredMutex     sync.RWMutex
	registeredProtocols = map[string]map[uint16]string{}
)

func noteProtocol(server string, protocolID uint16, name string) {
	registeredMutex.Lock()
	defer registeredMutex.Unlock()

	if registeredProtocols[server] == nil {
		registeredProtocols[server] = map[uint16]string{}
	}
	registeredProtocols[server][protocolID] = name
}

func protocolName(server string, protocolID uint16) (string, bool) {
	registeredMutex.RLock()
	defer registeredMutex.RUnlock()

	name, ok := registeredProtocols[server][protocolID]
	return name, ok
}

// DescribeProtocols renders the registered protocols of a server for logging.
func DescribeProtocols(server string) string {
	registeredMutex.RLock()
	defer registeredMutex.RUnlock()

	entries := registeredProtocols[server]
	ids := make([]int, 0, len(entries))
	for id := range entries {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)

	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("0x%02X %s", id, entries[uint16(id)]))
	}
	return strings.Join(parts, ", ")
}

// libraryVersion returns the NEX library version the servers present.
//
// Unverified: no public data exists for this title. Override with
// PN_SONIC3DS_NEX_VERSION (major.minor.patch) once a real capture pins it down.
func libraryVersion() *nex.LibraryVersion {
	major, minor, patch := 2, 0, 0

	if raw := os.Getenv("PN_SONIC3DS_NEX_VERSION"); raw != "" {
		parts := strings.Split(raw, ".")
		if len(parts) == 3 {
			if m, err := strconv.Atoi(parts[0]); err == nil {
				major = m
			}
			if m, err := strconv.Atoi(parts[1]); err == nil {
				minor = m
			}
			if m, err := strconv.Atoi(parts[2]); err == nil {
				patch = m
			}
		}
	}

	return nex.NewLibraryVersion(major, minor, patch)
}

// attachObservers installs shared logging hooks: unregistered protocols are
// always logged (this title has no known protocol list yet), and every RMC
// call is logged when PN_SONIC3DS_VERBOSE_PACKET_LOGGING=1.
func attachObservers(server string, endpoint *nex.PRUDPEndPoint) {
	verbose := os.Getenv("PN_SONIC3DS_VERBOSE_PACKET_LOGGING") == "1"

	endpoint.OnData(func(packet nex.PacketInterface) {
		request := packet.RMCMessage()
		if request == nil {
			return
		}

		if !request.IsRequest {
			if verbose {
				globals.Logger.Infof("[%s] response to protocol 0x%02X method 0x%02X from PID %d",
					server, request.ProtocolID, request.MethodID, packet.Sender().PID())
			}
			return
		}

		name, known := protocolName(server, request.ProtocolID)
		if !known {
			globals.Logger.Warningf(
				"[%s] unregistered protocol 0x%02X method 0x%02X from PID %d - Sonic 3DS needs this protocol added",
				server, request.ProtocolID, request.MethodID, packet.Sender().PID(),
			)
			return
		}

		if verbose {
			globals.Logger.Infof("[%s] %s(0x%02X)::method 0x%02X from PID %d",
				server, name, request.ProtocolID, request.MethodID, packet.Sender().PID())
		}
	})

	endpoint.OnError(func(err *nex.Error) {
		globals.Logger.Errorf("[%s] %s", server, err.Error())
	})
}

func StartAuthenticationServer(wg *sync.WaitGroup) {
	defer wg.Done()

	globals.AuthenticationServer = nex.NewPRUDPServer()
	globals.AuthenticationServer.ByteStreamSettings.UseStructureHeader = false
	globals.AuthenticationServer.SessionKeyLength = 32
	globals.AuthenticationServer.LibraryVersions.SetDefault(libraryVersion())
	globals.AuthenticationServer.AccessKey = accessKey()


	globals.AuthenticationEndpoint = nex.NewPRUDPEndPoint(1)
	globals.AuthenticationEndpoint.ServerAccount = globals.AuthenticationServerAccount
	globals.AuthenticationEndpoint.AccountDetailsByPID = globals.AccountDetailsByPID
	globals.AuthenticationEndpoint.AccountDetailsByUsername = globals.AccountDetailsByUsername


	globals.AuthenticationServer.BindPRUDPEndPoint(globals.AuthenticationEndpoint)

	attachObservers("auth", globals.AuthenticationEndpoint)
	registerAuthenticationProtocols()

	port, _ := strconv.Atoi(os.Getenv("PN_SONIC3DS_AUTHENTICATION_SERVER_PORT"))
	globals.Logger.Successf("Authentication server listening on UDP :%d (protocols: %s)", port, DescribeProtocols("auth"))
	globals.AuthenticationServer.Listen(port)
}

func StartSecureServer(wg *sync.WaitGroup) {
	defer wg.Done()

	globals.SecureServer = nex.NewPRUDPServer()
	globals.SecureServer.ByteStreamSettings.UseStructureHeader = false
	globals.SecureServer.SessionKeyLength = 32
	globals.SecureServer.LibraryVersions.SetDefault(libraryVersion())
	globals.SecureServer.AccessKey = accessKey()

	globals.SecureEndpoint = nex.NewPRUDPEndPoint(1)
	globals.SecureEndpoint.IsSecureEndPoint = true
	globals.SecureEndpoint.ServerAccount = globals.SecureServerAccount
	globals.SecureEndpoint.AccountDetailsByPID = globals.AccountDetailsByPID
	globals.SecureEndpoint.AccountDetailsByUsername = globals.AccountDetailsByUsername
	globals.SecureServer.BindPRUDPEndPoint(globals.SecureEndpoint)

	attachObservers("secure", globals.SecureEndpoint)
	registerSecureProtocols()

	port, _ := strconv.Atoi(os.Getenv("PN_SONIC3DS_SECURE_SERVER_PORT"))
	globals.Logger.Successf("Secure server listening on UDP :%d (protocols: %s)", port, DescribeProtocols("secure"))
	globals.SecureServer.Listen(port)
}
