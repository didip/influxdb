package coordinator

import (
	"cluster"
	"encoding/json"
	"io"
	"time"

	log "code.google.com/p/log4go"
	"github.com/goraft/raft"
)

var internalRaftCommands map[string]raft.Command

func init() {
	internalRaftCommands = map[string]raft.Command{}
	for _, command := range []raft.Command{
		&AddPotentialServerCommand{},
		&CreateDatabaseCommand{},
		&DropDatabaseCommand{},
		&SaveDbUserCommand{},
		&SaveClusterAdminCommand{},
		&ChangeDbUserPassword{},
		&CreateContinuousQueryCommand{},
		&DeleteContinuousQueryCommand{},
		&SetContinuousQueryTimestampCommand{},
		&CreateShardsCommand{},
		&DropShardCommand{},
	} {
		internalRaftCommands[command.CommandName()] = command
	}
}

type SetContinuousQueryTimestampCommand struct {
	Timestamp time.Time `json:"timestamp"`
}

func NewSetContinuousQueryTimestampCommand(timestamp time.Time) *SetContinuousQueryTimestampCommand {
	return &SetContinuousQueryTimestampCommand{timestamp}
}

func (c *SetContinuousQueryTimestampCommand) CommandName() string {
	return "set_cq_ts"
}

func (c *SetContinuousQueryTimestampCommand) Apply(server raft.Server) (interface{}, error) {
	config := server.Context().(*cluster.ClusterConfiguration)
	err := config.SetContinuousQueryTimestamp(c.Timestamp)
	return nil, err
}

type CreateContinuousQueryCommand struct {
	Database string `json:"database"`
	Query    string `json:"query"`
}

func NewCreateContinuousQueryCommand(database string, query string) *CreateContinuousQueryCommand {
	return &CreateContinuousQueryCommand{database, query}
}

func (c *CreateContinuousQueryCommand) CommandName() string {
	return "create_cq"
}

func (c *CreateContinuousQueryCommand) Apply(server raft.Server) (interface{}, error) {
	config := server.Context().(*cluster.ClusterConfiguration)
	err := config.CreateContinuousQuery(c.Database, c.Query)
	return nil, err
}

type DeleteContinuousQueryCommand struct {
	Database string `json:"database"`
	Id       uint32 `json:"id"`
}

func NewDeleteContinuousQueryCommand(database string, id uint32) *DeleteContinuousQueryCommand {
	return &DeleteContinuousQueryCommand{database, id}
}

func (c *DeleteContinuousQueryCommand) CommandName() string {
	return "delete_cq"
}

func (c *DeleteContinuousQueryCommand) Apply(server raft.Server) (interface{}, error) {
	config := server.Context().(*cluster.ClusterConfiguration)
	err := config.DeleteContinuousQuery(c.Database, c.Id)
	return nil, err
}

type DropDatabaseCommand struct {
	Name string `json:"name"`
}

func NewDropDatabaseCommand(name string) *DropDatabaseCommand {
	return &DropDatabaseCommand{name}
}

func (c *DropDatabaseCommand) CommandName() string {
	return "drop_db"
}

func (c *DropDatabaseCommand) Apply(server raft.Server) (interface{}, error) {
	config := server.Context().(*cluster.ClusterConfiguration)
	err := config.DropDatabase(c.Name)
	return nil, err
}

type CreateDatabaseCommand struct {
	Name              string `json:"name"`
	ReplicationFactor uint8  `json:"replicationFactor"`
}

func NewCreateDatabaseCommand(name string, replicationFactor uint8) *CreateDatabaseCommand {
	return &CreateDatabaseCommand{name, replicationFactor}
}

func (c *CreateDatabaseCommand) CommandName() string {
	return "create_db"
}

func (c *CreateDatabaseCommand) Apply(server raft.Server) (interface{}, error) {
	config := server.Context().(*cluster.ClusterConfiguration)
	err := config.CreateDatabase(c.Name, c.ReplicationFactor)
	return nil, err
}

type SaveDbUserCommand struct {
	User *cluster.DbUser `json:"user"`
}

func NewSaveDbUserCommand(u *cluster.DbUser) *SaveDbUserCommand {
	return &SaveDbUserCommand{
		User: u,
	}
}

func (c *SaveDbUserCommand) CommandName() string {
	return "save_db_user"
}

func (c *SaveDbUserCommand) Apply(server raft.Server) (interface{}, error) {
	config := server.Context().(*cluster.ClusterConfiguration)
	config.SaveDbUser(c.User)
	log.Debug("(raft:%s) Created user %s:%s", server.Name(), c.User.Db, c.User.Name)
	return nil, nil
}

type ChangeDbUserPassword struct {
	Database string
	Username string
	Hash     string
}

func NewChangeDbUserPasswordCommand(db, username, hash string) *ChangeDbUserPassword {
	return &ChangeDbUserPassword{
		Database: db,
		Username: username,
		Hash:     hash,
	}
}

func (c *ChangeDbUserPassword) CommandName() string {
	return "change_db_user_password"
}

func (c *ChangeDbUserPassword) Apply(server raft.Server) (interface{}, error) {
	log.Debug("(raft:%s) changing db user password for %s:%s", server.Name(), c.Database, c.Username)
	config := server.Context().(*cluster.ClusterConfiguration)
	return nil, config.ChangeDbUserPassword(c.Database, c.Username, c.Hash)
}

type SaveClusterAdminCommand struct {
	User *cluster.ClusterAdmin `json:"user"`
}

func NewSaveClusterAdminCommand(u *cluster.ClusterAdmin) *SaveClusterAdminCommand {
	return &SaveClusterAdminCommand{
		User: u,
	}
}

func (c *SaveClusterAdminCommand) CommandName() string {
	return "save_cluster_admin_user"
}

func (c *SaveClusterAdminCommand) Apply(server raft.Server) (interface{}, error) {
	config := server.Context().(*cluster.ClusterConfiguration)
	config.SaveClusterAdmin(c.User)
	return nil, nil
}

type AddPotentialServerCommand struct {
	Server *cluster.ClusterServer
}

func NewAddPotentialServerCommand(s *cluster.ClusterServer) *AddPotentialServerCommand {
	return &AddPotentialServerCommand{Server: s}
}

func (c *AddPotentialServerCommand) CommandName() string {
	return "add_server"
}

func (c *AddPotentialServerCommand) Apply(server raft.Server) (interface{}, error) {
	config := server.Context().(*cluster.ClusterConfiguration)
	config.AddPotentialServer(c.Server)
	return nil, nil
}

type InfluxJoinCommand struct {
	Name                     string `json:"name"`
	ConnectionString         string `json:"connectionString"`
	ProtobufConnectionString string `json:"protobufConnectionString"`
}

// The name of the Join command in the log
func (c *InfluxJoinCommand) CommandName() string {
	return "raft:join"
}

func (c *InfluxJoinCommand) Apply(server raft.Server) (interface{}, error) {
	err := server.AddPeer(c.Name, c.ConnectionString)

	return []byte("join"), err
}

func (c *InfluxJoinCommand) NodeName() string {
	return c.Name
}

type CreateShardsCommand struct {
	Shards []*cluster.NewShardData
}

func NewCreateShardsCommand(shards []*cluster.NewShardData) *CreateShardsCommand {
	return &CreateShardsCommand{shards}
}

func (c *CreateShardsCommand) CommandName() string {
	return "create_shards"
}

// TODO: Encode/Decode are not needed once this pr
// https://github.com/goraft/raft/pull/221 is merged in and our goraft
// is updated to a commit that includes the pr

func (c *CreateShardsCommand) Encode(w io.Writer) error {
	return json.NewEncoder(w).Encode(c)
}
func (c *CreateShardsCommand) Decode(r io.Reader) error {
	return json.NewDecoder(r).Decode(c)
}

func (c *CreateShardsCommand) Apply(server raft.Server) (interface{}, error) {
	config := server.Context().(*cluster.ClusterConfiguration)
	createdShards, err := config.AddShards(c.Shards)
	if err != nil {
		return nil, err
	}
	createdShardData := make([]*cluster.NewShardData, 0)
	for _, s := range createdShards {
		createdShardData = append(createdShardData, s.ToNewShardData())
	}
	return createdShardData, nil
}

type DropShardCommand struct {
	ShardId   uint32
	ServerIds []uint32
}

func NewDropShardCommand(id uint32, serverIds []uint32) *DropShardCommand {
	return &DropShardCommand{ShardId: id, ServerIds: serverIds}
}

func (c *DropShardCommand) CommandName() string {
	return "drop_shard"
}

func (c *DropShardCommand) Apply(server raft.Server) (interface{}, error) {
	config := server.Context().(*cluster.ClusterConfiguration)
	err := config.DropShard(c.ShardId, c.ServerIds)
	return nil, err
}
