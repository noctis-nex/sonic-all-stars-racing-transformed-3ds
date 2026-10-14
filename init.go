package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	pb "github.com/PretendoNetwork/grpc/go/account/v2"
	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	"github.com/PretendoNetwork/plogger-go"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/Protarium-Network/sonic-all-stars-racing-transformed-3ds/globals"
)

func init() {
	globals.Logger = plogger.NewLogger()

	if err := godotenv.Load(); err != nil {
		globals.Logger.Warning("Error loading .env file")
	}

	postgresURI := os.Getenv("PN_SONIC3DS_POSTGRES_URI")
	authenticationServerPort := os.Getenv("PN_SONIC3DS_AUTHENTICATION_SERVER_PORT")
	secureServerPort := os.Getenv("PN_SONIC3DS_SECURE_SERVER_PORT")
	accountGRPCHost := os.Getenv("PN_SONIC3DS_ACCOUNT_GRPC_HOST")
	accountGRPCPort := os.Getenv("PN_SONIC3DS_ACCOUNT_GRPC_PORT")
	accountGRPCAPIKey := os.Getenv("PN_SONIC3DS_ACCOUNT_GRPC_API_KEY")
	gameAccessKey := strings.TrimSpace(os.Getenv("PN_SONIC3DS_ACCESS_KEY"))

	if strings.TrimSpace(postgresURI) == "" {
		globals.Logger.Error("PN_SONIC3DS_POSTGRES_URI environment variable not set")
		os.Exit(1)
	}

	kerberosPassword := make([]byte, 0x10)
	if _, err := rand.Read(kerberosPassword); err != nil {
		globals.Logger.Error("Error generating Kerberos password")
		os.Exit(1)
	}
	globals.KerberosPassword = string(kerberosPassword)
	globals.AuthenticationServerAccount = nex.NewAccount(types.NewPID(1), "Quazal Authentication", globals.KerberosPassword, false)
	globals.SecureServerAccount = nex.NewAccount(types.NewPID(2), "Quazal Rendez-Vous", globals.KerberosPassword, false)

	if strings.TrimSpace(authenticationServerPort) == "" {
		globals.Logger.Error("PN_SONIC3DS_AUTHENTICATION_SERVER_PORT environment variable not set")
		os.Exit(1)
	}
	if _, err := strconv.Atoi(authenticationServerPort); err != nil {
		globals.Logger.Errorf("PN_SONIC3DS_AUTHENTICATION_SERVER_PORT is not a valid port: %s", authenticationServerPort)
		os.Exit(1)
	}

	if strings.TrimSpace(secureServerPort) == "" {
		globals.Logger.Error("PN_SONIC3DS_SECURE_SERVER_PORT environment variable not set")
		os.Exit(1)
	}
	if _, err := strconv.Atoi(secureServerPort); err != nil {
		globals.Logger.Errorf("PN_SONIC3DS_SECURE_SERVER_PORT is not a valid port: %s", secureServerPort)
		os.Exit(1)
	}

	if strings.TrimSpace(accountGRPCHost) == "" || strings.TrimSpace(accountGRPCPort) == "" {
		globals.Logger.Error("PN_SONIC3DS_ACCOUNT_GRPC_HOST / PN_SONIC3DS_ACCOUNT_GRPC_PORT environment variables not set")
		os.Exit(1)
	}
	if strings.TrimSpace(accountGRPCAPIKey) == "" {
		globals.Logger.Error("PN_SONIC3DS_ACCOUNT_GRPC_API_KEY environment variable not set")
		os.Exit(1)
	}
	if gameAccessKey == "" {
		globals.Logger.Error("PN_SONIC3DS_ACCESS_KEY environment variable not set")
		os.Exit(1)
	}

	var err error
	globals.GRPCAccountClientConnection, err = grpc.NewClient(
		fmt.Sprintf("%s:%s", accountGRPCHost, accountGRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		globals.Logger.Criticalf("Failed to connect to account gRPC server: %v", err)
		os.Exit(1)
	}
	globals.GRPCAccountClient = pb.NewAccountServiceClient(globals.GRPCAccountClientConnection)
	globals.GRPCAccountCommonMetadata = metadata.Pairs("X-API-Key", accountGRPCAPIKey)
	// The shared SecureConnection protocol validates 3DS tokens through its own
	// globals package. Give it the same initialized client and credentials.
	common_globals.GRPCAccountClientConnection = globals.GRPCAccountClientConnection
	common_globals.GRPCAccountClient = globals.GRPCAccountClient
	common_globals.GRPCAccountCommonMetadata = globals.GRPCAccountCommonMetadata

	globals.Postgres, err = sql.Open("postgres", postgresURI)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}
	if err := globals.Postgres.Ping(); err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}
	globals.Logger.Success("Connected to Postgres!")
}
