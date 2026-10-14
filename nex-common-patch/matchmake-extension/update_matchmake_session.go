package matchmake_extension

import (
	"github.com/PretendoNetwork/nex-go/v2"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	"github.com/PretendoNetwork/nex-protocols-common-go/v2/matchmake-extension/database"
	match_making_types "github.com/PretendoNetwork/nex-protocols-go/v2/match-making/types"
	matchmake_extension "github.com/PretendoNetwork/nex-protocols-go/v2/matchmake-extension"
	pqextended "github.com/PretendoNetwork/pq-extended"
)

// updateMatchmakeSession changes client-owned session settings while preserving
// the server-assigned identity, participants, owner, host, and session key.
func (commonProtocol *CommonProtocol) updateMatchmakeSession(err error, packet nex.PacketInterface, callID uint32, anyGathering match_making_types.GatheringHolder) (*nex.RMCMessage, *nex.Error) {
	if err != nil {
		common_globals.Logger.Error(err.Error())
		return nil, nex.NewError(nex.ResultCodes.Core.InvalidArgument, "change_error")
	}
	session, ok := anyGathering.Object.(match_making_types.MatchmakeSession)
	if !ok || !common_globals.CheckValidMatchmakeSession(session) || session.Gathering.ID == 0 {
		return nil, nex.NewError(nex.ResultCodes.Core.InvalidArgument, "change_error")
	}

	connection := packet.Sender().(*nex.PRUDPConnection)
	endpoint := connection.Endpoint().(*nex.PRUDPEndPoint)
	commonProtocol.manager.Mutex.Lock()
	stored, _, nexError := database.GetMatchmakeSessionByID(commonProtocol.manager, endpoint, uint32(session.Gathering.ID))
	if nexError != nil {
		commonProtocol.manager.Mutex.Unlock()
		return nil, nexError
	}
	if !stored.Gathering.OwnerPID.Equals(connection.PID()) {
		commonProtocol.manager.Mutex.Unlock()
		return nil, nex.NewError(nex.ResultCodes.RendezVous.PermissionDenied, "change_error")
	}

	attribs := make([]uint32, len(session.Attributes))
	for i, attribute := range session.Attributes {
		attribs[i] = uint32(attribute)
	}
	tx, dbErr := commonProtocol.manager.Database.Begin()
	if dbErr == nil {
		_, dbErr = tx.Exec(`UPDATE matchmaking.gatherings SET min_participants=$1, max_participants=$2,
			participation_policy=$3, policy_argument=$4, flags=$5, state=$6, description=$7
			WHERE id=$8 AND registered=true AND type='MatchmakeSession'`,
			session.Gathering.MinimumParticipants, session.Gathering.MaximumParticipants,
			session.Gathering.ParticipationPolicy, session.Gathering.PolicyArgument,
			session.Gathering.Flags, session.Gathering.State, session.Gathering.Description, session.Gathering.ID)
	}
	if dbErr == nil {
		_, dbErr = tx.Exec(`UPDATE matchmaking.matchmake_sessions SET game_mode=$1, attribs=$2,
			open_participation=$3, matchmake_system_type=$4, application_buffer=$5 WHERE id=$6`,
			session.GameMode, pqextended.Array(attribs), session.OpenParticipation,
			session.MatchmakeSystemType, []byte(session.ApplicationBuffer), session.Gathering.ID)
	}
	if dbErr != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		commonProtocol.manager.Mutex.Unlock()
		return nil, nex.NewError(nex.ResultCodes.Core.Unknown, dbErr.Error())
	}
	if dbErr = tx.Commit(); dbErr != nil {
		commonProtocol.manager.Mutex.Unlock()
		return nil, nex.NewError(nex.ResultCodes.Core.Unknown, dbErr.Error())
	}
	commonProtocol.manager.Mutex.Unlock()

	response := nex.NewRMCSuccess(endpoint, nil)
	response.ProtocolID = matchmake_extension.ProtocolID
	response.MethodID = matchmake_extension.MethodUpdateMatchmakeSession
	response.CallID = callID
	return response, nil
}
