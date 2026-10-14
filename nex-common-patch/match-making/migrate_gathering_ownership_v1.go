package matchmaking

import (
	"slices"

	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	"github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making/database"
	"github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making/tracking"
	match_making "github.com/PretendoNetwork/nex-protocols-go/v2/match-making"
	notifications_constants "github.com/PretendoNetwork/nex-protocols-go/v2/notifications/constants"
	notifications_types "github.com/PretendoNetwork/nex-protocols-go/v2/notifications/types"
)

// migrateGatheringOwnershipV1 elects the first requested participant as owner.
func (commonProtocol *CommonProtocol) migrateGatheringOwnershipV1(err error, packet nex.PacketInterface, callID uint32, gid types.UInt32, candidates types.List[types.PID]) (*nex.RMCMessage, *nex.Error) {
	if err != nil {
		common_globals.Logger.Error(err.Error())
		return nil, nex.NewError(nex.ResultCodes.Core.InvalidArgument, "change_error")
	}

	connection := packet.Sender().(*nex.PRUDPConnection)
	endpoint := connection.Endpoint().(*nex.PRUDPEndPoint)
	commonProtocol.manager.Mutex.Lock()
	gathering, _, participants, _, nexError := database.FindGatheringByID(commonProtocol.manager, uint32(gid))
	if nexError != nil {
		commonProtocol.manager.Mutex.Unlock()
		return nil, nexError
	}
	if !gathering.OwnerPID.Equals(connection.PID()) && !gathering.HostPID.Equals(connection.PID()) {
		commonProtocol.manager.Mutex.Unlock()
		return nil, nex.NewError(nex.ResultCodes.RendezVous.PermissionDenied, "change_error")
	}

	var newOwner uint64
	for _, candidate := range candidates {
		if uint64(candidate) != uint64(gathering.OwnerPID) && slices.Contains(participants, uint64(candidate)) {
			newOwner = uint64(candidate)
			break
		}
	}
	if newOwner != 0 {
		newOwnerPID := types.NewPID(newOwner)
		nexError = database.UpdateSessionHost(commonProtocol.manager, uint32(gid), newOwnerPID, gathering.HostPID)
		if nexError == nil {
			nexError = tracking.LogChangeOwner(commonProtocol.manager.Database, connection.PID(), uint32(gid), gathering.OwnerPID, newOwnerPID)
		}
		if nexError != nil {
			commonProtocol.manager.Mutex.Unlock()
			return nil, nexError
		}
		event := notifications_types.NewNotificationEvent()
		event.PIDSource = connection.PID()
		event.Type = notifications_constants.NotificationCategoryOwnershipChangeEvent.Build()
		event.Param1 = types.UInt64(gid)
		event.Param2 = types.UInt64(newOwner)
		common_globals.SendNotificationEvent(endpoint, event, common_globals.RemoveDuplicates(participants))
	}
	commonProtocol.manager.Mutex.Unlock()

	stream := nex.NewByteStreamOut(endpoint.LibraryVersions(), endpoint.ByteStreamSettings())
	types.NewBool(newOwner != 0).WriteTo(stream)
	response := nex.NewRMCSuccess(endpoint, stream.Bytes())
	response.ProtocolID = match_making.ProtocolID
	response.MethodID = match_making.MethodMigrateGatheringOwnershipV1
	response.CallID = callID
	return response, nil
}
