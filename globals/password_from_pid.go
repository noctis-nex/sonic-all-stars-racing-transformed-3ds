package globals

import (
	"context"
	"time"

	pb "github.com/PretendoNetwork/grpc/go/account/v2"
	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	"google.golang.org/grpc/metadata"
)

const grpcTimeout = 5 * time.Second

// PasswordFromPID asks pn-account for a PID's real NEX password over gRPC.
//
// This only returns a real password once the console's own real Pretendo
// password has been registered for that PID via the Protarium Connect
// homebrew app (FRD_GetMyPassword bridge) and nasc-router's /register
// endpoint - the same mechanism already used by the 3DS Super Mario Maker
// server.
func PasswordFromPID(pid types.PID) (string, uint32) {
	ctx, cancel := context.WithTimeout(context.Background(), grpcTimeout)
	defer cancel()

	ctx = metadata.NewOutgoingContext(ctx, GRPCAccountCommonMetadata)

	response, err := GRPCAccountClient.GetNEXPassword(ctx, &pb.GetNEXPasswordRequest{Pid: uint32(pid)})
	if err != nil {
		Logger.Errorf("Failed to get NEX password for PID %d: %v", uint32(pid), err)
		return "", nex.ResultCodes.RendezVous.InvalidUsername
	}

	return response.Password, 0
}
