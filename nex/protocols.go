package nex

import (
	"crypto/rand"
	"fmt"

	nex "github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/constants"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	common_match_making "github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making"
	common_match_making_ext "github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making-ext"
	common_matchmake_extension "github.com/PretendoNetwork/nex-protocols-common-go/v2/matchmake-extension"
	common_nat_traversal "github.com/PretendoNetwork/nex-protocols-common-go/v2/nat-traversal"
	common_secure_connection "github.com/PretendoNetwork/nex-protocols-common-go/v2/secure-connection"
	common_ticket_granting "github.com/PretendoNetwork/nex-protocols-common-go/v2/ticket-granting"
	match_making "github.com/PretendoNetwork/nex-protocols-go/v2/match-making"
	match_making_ext "github.com/PretendoNetwork/nex-protocols-go/v2/match-making-ext"
	match_making_types "github.com/PretendoNetwork/nex-protocols-go/v2/match-making/types"
	matchmake_extension "github.com/PretendoNetwork/nex-protocols-go/v2/matchmake-extension"
	nat_traversal "github.com/PretendoNetwork/nex-protocols-go/v2/nat-traversal"
	secure_connection "github.com/PretendoNetwork/nex-protocols-go/v2/secure-connection"
	ticket_granting "github.com/PretendoNetwork/nex-protocols-go/v2/ticket-granting"
	ticket_granting_types "github.com/PretendoNetwork/nex-protocols-go/v2/ticket-granting/types"

	"github.com/Protarium-Network/sonic-all-stars-racing-transformed-3ds/globals"
)

// GameServerID is the game_server_id accepted by this implementation's
// account-token validation.
const GameServerID = "0008FC00"

// loginDataValidator exchanges the console's NEX token with pn-account over
// gRPC to prove it was issued for this game_server_id and belongs to the
// claimed PID - the same check every other self-hosted 3DS/Wii U game on
// this server uses.
func loginDataValidator() func(types.PID, types.DataHolder) *nex.Error {
	return func(pid types.PID, loginData types.DataHolder) *nex.Error {
		return common_globals.ValidatePNLoginData(pid, loginData, []string{GameServerID})
	}
}

func secureStationURL(secureServerHost string, secureServerPort int) types.StationURL {
	url := types.NewStationURL("")

	url.SetURLType(constants.StationURLPRUDPS)
	url.SetAddress(secureServerHost)
	url.SetPortNumber(uint16(secureServerPort))
	url.SetConnectionID(1)
	url.SetPrincipalID(types.NewPID(2))
	url.SetStreamID(1)
	url.SetStreamType(constants.StreamTypeRVSecure)

	// Advertise this endpoint as public. Validate this flag against captures
	// from the target game before changing it.
	url.SetType(uint8(constants.StationURLFlagPublic))

	return url
}

func registerAuthenticationProtocols() {
	protocol := ticket_granting.NewProtocol()
	globals.AuthenticationEndpoint.RegisterServiceProtocol(protocol)
	noteProtocol("auth", ticket_granting.ProtocolID, "TicketGranting")

	common := common_ticket_granting.NewCommonProtocol(protocol)
	// NEX 2 game-server tickets use a 32-byte session key. A 16-byte key is
	// reserved for NEX 1 and the 3DS/Wii U friends service.
	common.SessionKeyLength = 32
	// The NEX client reads the server version from LoginEx's build string.
	// Keep this labeled as our implementation rather than impersonating an
	// original Nintendo build.
	// Sonic's EUR client compares the second underscore-delimited component
	// with 21 and reports ServerVersionIsOld when it is 21 or lower.
	common.BuildName = types.NewString("branch:origin/protarium/sonic3ds build:2_22_0_1_0")

	// The console calls LoginEx (method 0x02), which validates its NASC token.
	// Leave the unauthenticated Login method disabled.

	common.SecureServerAccount = globals.SecureServerAccount
	common.ValidateLoginData = loginDataValidator()
	originalLoginEx := protocol.LoginEx
	protocol.SetHandlerLoginEx(func(err error, packet nex.PacketInterface, callID uint32, strUserName types.String, extraData types.DataHolder) (*nex.RMCMessage, *nex.Error) {
		if info, ok := extraData.Object.(ticket_granting_types.AuthenticationInfo); ok {
			globals.Logger.Infof("[auth] LoginEx NGS version=%d", uint32(info.NGSVersion))
		}
		return originalLoginEx(err, packet, callID, strUserName, extraData)
	})

	// * Station URL host/port come from the same env vars main.go reads, via a
	// * package-level setter called from main before the servers start.
	common.SecureStationURL = pendingSecureStationURL

	// * Reverted: smm-server (confirmed-working 3DS NEX title, real console
	// * tested) never touches StationURLSpecialProtocols at all - it stays at
	// * the library default (empty StationURL). Populating it here was an
	// * unverified experiment from an earlier session; matching the
	// * proven-working reference means leaving it untouched.

	// Log the username field while validating the login flow. In normal use the
	// server expects this field to contain the decimal player ID.
	originalLogin := protocol.Login
	protocol.SetHandlerLogin(func(err error, packet nex.PacketInterface, callID uint32, strUserName types.String) (*nex.RMCMessage, *nex.Error) {
		globals.Logger.Infof("[auth] TicketGranting::Login strUserName=%q (err=%v)", string(strUserName), err)
		return originalLogin(err, packet, callID, strUserName)
	})

	// Preserve the library's RequestTicket implementation. It generates a real
	// Kerberos ticket and serializes the response for the configured NEX version.
	// Returning NotImplemented here prevents the client from ever reaching the
	// secure server, even when authentication itself succeeds.
	originalRequestTicket := protocol.RequestTicket
	protocol.SetHandlerRequestTicket(func(err error, packet nex.PacketInterface, callID uint32, idSource types.PID, idTarget types.PID) (*nex.RMCMessage, *nex.Error) {
		globals.Logger.Infof("[auth] TicketGranting::RequestTicket idSource=%d idTarget=%d (err=%v)", uint64(idSource), uint64(idTarget), err)
		response, nexErr := originalRequestTicket(err, packet, callID, idSource, idTarget)
		if nexErr != nil {
			globals.Logger.Warningf("[auth] RequestTicket failed: %v", nexErr)
		} else if response != nil {
			globals.Logger.Infof("[auth] RequestTicket response sent")
		}
		return response, nexErr
	})
}

// generateRichTicket mirrors nex-protocols-common-go's own unexported
// generateTicket (ticket-granting/generate_ticket.go) - duplicated here only
// because it isn't exported, not because the logic differs.
func generateRichTicket(source, target *nex.Account, endpoint *nex.PRUDPEndPoint) ([]byte, *nex.Error) {
	if source == nil || target == nil {
		return []byte{}, nex.NewError(nex.ResultCodes.Authentication.Unknown, "Source or target account is nil")
	}

	sourceKey := nex.DeriveKerberosKey(source.PID, []byte(source.Password))
	targetKey := nex.DeriveKerberosKey(target.PID, []byte(target.Password))

	sessionKey := make([]byte, 32)
	if _, err := rand.Read(sessionKey); err != nil {
		return []byte{}, nex.NewError(nex.ResultCodes.Authentication.Unknown, "Failed to generate session key")
	}

	ticketInternalData := nex.NewKerberosTicketInternalData(endpoint.Server)
	ticketInternalData.Issued = types.NewDateTime(0).Now()
	ticketInternalData.SourcePID = source.PID
	ticketInternalData.SessionKey = sessionKey

	encryptedTicketInternalData, err := ticketInternalData.Encrypt(targetKey, nex.NewByteStreamOut(endpoint.LibraryVersions(), endpoint.ByteStreamSettings()))
	if err != nil {
		return []byte{}, nex.NewError(nex.ResultCodes.Authentication.Unknown, "Failed to encrypt Ticket Internal Data")
	}

	ticket := nex.NewKerberosTicket()
	ticket.SessionKey = sessionKey
	ticket.TargetPID = target.PID
	ticket.InternalData = types.NewBuffer(encryptedTicketInternalData)

	encryptedTicket, err := ticket.Encrypt(sourceKey, nex.NewByteStreamOut(endpoint.LibraryVersions(), endpoint.ByteStreamSettings()))
	if err != nil {
		return []byte{}, nex.NewError(nex.ResultCodes.Authentication.Unknown, "Failed to encrypt Ticket")
	}

	return encryptedTicket, nil
}

// pendingSecureStationURL is set by SetSecureStationURL before the servers
// start, since the authentication server needs to advertise the secure
// server's externally-reachable address and that address is only known once
// the environment has been read.
var pendingSecureStationURL types.StationURL

// SetSecureStationURL configures the address the authentication server tells
// consoles to use for the secure server. Must be called before
// StartAuthenticationServer.
func SetSecureStationURL(host string, port int) {
	pendingSecureStationURL = secureStationURL(host, port)
}

// registerSecureProtocols installs the protocols currently implemented by
// this server. Unknown protocol IDs are logged for later investigation.
func registerSecureProtocols() {
	registerSecureConnection()
	registerNATTraversal()
	registerMatchmaking()
}

func registerSecureConnection() {
	protocol := secure_connection.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(protocol)
	noteProtocol("secure", secure_connection.ProtocolID, "SecureConnection")

	common := common_secure_connection.NewCommonProtocol(protocol)

	// * Register (method 1) carries no login data on its own; the PID here comes
	// * from the Kerberos ticket the console already holds by the time it reaches
	// * this protocol, which is what actually gates access.
	common.EnableInsecureRegister()
	common.ValidateLoginData = loginDataValidator()
	originalRegister := protocol.Register
	protocol.SetHandlerRegister(func(err error, packet nex.PacketInterface, callID uint32, urls types.List[types.StationURL]) (*nex.RMCMessage, *nex.Error) {
		response, nexErr := originalRegister(err, packet, callID, urls)
		if nexErr != nil {
			globals.Logger.Warningf("[secure] Register error: %v", nexErr)
		} else if response != nil {
			globals.Logger.Infof("[secure] Register response success=%t code=0x%08X params=%d urls=%d", response.IsSuccess, response.ErrorCode, len(response.Parameters), len(urls))
		}
		return response, nexErr
	})
	common.OnAfterRegisterEx = func(packet nex.PacketInterface, urls types.List[types.StationURL], custom types.DataHolder) {
		connection := packet.Sender().(*nex.PRUDPConnection)
		common_globals.Logger.Info(fmt.Sprintf("SONIC3DS register pid=%d input=%v stored=%v", connection.PID(), urls, connection.StationURLs))
	}
}

func registerNATTraversal() {
	protocol := nat_traversal.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(protocol)
	noteProtocol("secure", nat_traversal.ProtocolID, "NATTraversal")

	sonicNAT := common_nat_traversal.NewCommonProtocol(protocol)
	sonicNAT.OnAfterRequestProbeInitiationExt = func(packet nex.PacketInterface, targets types.List[types.String], station types.String) {
		pid := packet.Sender().(*nex.PRUDPConnection).PID()
		common_globals.Logger.Info(fmt.Sprintf("SONIC3DS NAT pid=%d targets=%v station=%s", pid, targets, station))
	}
	sonicNAT.OnAfterReportNATTraversalResult = func(packet nex.PacketInterface, cid types.UInt32, result types.Bool, rtt types.UInt32) {
		pid := packet.Sender().(*nex.PRUDPConnection).PID()
		common_globals.Logger.Info(fmt.Sprintf("SONIC3DS NAT result pid=%d cid=%d success=%t rtt=%d", pid, cid, result, rtt))
	}
}

func registerMatchmaking() {
	globals.MatchmakingManager = common_globals.NewMatchmakingManager(globals.SecureEndpoint, globals.Postgres)

	matchMakingProtocol := match_making.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(matchMakingProtocol)
	noteProtocol("secure", match_making.ProtocolID, "MatchMaking")
	common_match_making.NewCommonProtocol(matchMakingProtocol).SetManager(globals.MatchmakingManager)

	matchMakingExtProtocol := match_making_ext.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(matchMakingExtProtocol)
	noteProtocol("secure", match_making_ext.ProtocolID, "MatchMakingExt")
	common_match_making_ext.NewCommonProtocol(matchMakingExtProtocol).SetManager(globals.MatchmakingManager)

	matchmakeExtensionProtocol := matchmake_extension.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(matchmakeExtensionProtocol)
	noteProtocol("secure", matchmake_extension.ProtocolID, "MatchmakeExtension")
	sonicMatchmaking := common_matchmake_extension.NewCommonProtocol(matchmakeExtensionProtocol)
	sonicMatchmaking.SetManager(globals.MatchmakingManager)
	sonicMatchmaking.OnAfterBrowseMatchmakeSession = func(packet nex.PacketInterface, criteria match_making_types.MatchmakeSessionSearchCriteria, resultRange types.ResultRange) {
		pid := packet.Sender().(*nex.PRUDPConnection).PID()
		common_globals.Logger.Info(fmt.Sprintf("SONIC3DS search pid=%d mode=%q min=%q max=%q attribs=%v vacant=%t locked=%t host=%t", pid, criteria.GameMode, criteria.MinParticipants, criteria.MaxParticipants, criteria.Attribs, criteria.VacantOnly, criteria.ExcludeLocked, criteria.ExcludeNonHostPID))
	}
}
