package globals

import (
	"database/sql"

	pb "github.com/PretendoNetwork/grpc/go/account/v2"
	"github.com/PretendoNetwork/nex-go/v2"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	"github.com/PretendoNetwork/plogger-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var Postgres *sql.DB

var Logger *plogger.Logger
var KerberosPassword string

var AuthenticationServer *nex.PRUDPServer
var AuthenticationEndpoint *nex.PRUDPEndPoint
var SecureServer *nex.PRUDPServer
var SecureEndpoint *nex.PRUDPEndPoint

var MatchmakingManager *common_globals.MatchmakingManager

var GRPCAccountClientConnection *grpc.ClientConn
var GRPCAccountClient pb.AccountServiceClient
var GRPCAccountCommonMetadata metadata.MD
