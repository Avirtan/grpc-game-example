package client

import (
	"io"
	"log"

	"github.com/google/uuid"
	"github.com/mortenson/grpc-game-example/pkg/backend"
	"github.com/mortenson/grpc-game-example/pkg/frontend"
	"github.com/mortenson/grpc-game-example/proto"
)

// GameClient is used to stream game information to a server and update the
// game state as needed.
type GameClient struct {
	CurrentPlayer uuid.UUID
	Stream        proto.Game_StreamClient
	Game          *backend.Game
	View          *frontend.View
}

// NewGameClient constructs a new game client struct.
func NewGameClient(stream proto.Game_StreamClient, game *backend.Game, view *frontend.View) *GameClient {
	return &GameClient{
		Stream: stream,
		Game:   game,
		View:   view,
	}
}

// Connect connects a new player to the server.
func (c *GameClient) Connect(playerID uuid.UUID, playerName string) {
	c.View.Paused = true
	c.CurrentPlayer = playerID
	req := proto.Request{
		Action: &proto.Request_Connect{
			Connect: &proto.Connect{
				Id:   playerID.String(),
				Name: playerName,
			},
		},
	}
	c.Stream.Send(&req)
}

// Start begins the goroutines needed to recieve server changes and send game
// changes.
func (c *GameClient) Start() {
	// Handle local game engine changes.
	go func() {
		for {
			change := <-c.Game.ChangeChannel
			switch change.(type) {
			case backend.MoveChange:
				change := change.(backend.MoveChange)
				c.HandleMoveChange(change)
			case backend.AddEntityChange:
				change := change.(backend.AddEntityChange)
				c.HandleAddEntityChange(change)
			}
		}
	}()
	// Handle stream messages.
	go func() {
		for {
			resp, err := c.Stream.Recv()
			if err == io.EOF {
				log.Fatalf("EOF")
				return
			}
			if err != nil {
				log.Fatalf("can not receive %v", err)
			}

			switch resp.GetAction().(type) {
			case *proto.Response_Initialize:
				c.HandleInitializeResponse(resp)
			case *proto.Response_AddEntity:
				c.HandleAddEntityResponse(resp)
			case *proto.Response_UpdateEntity:
				c.HandleUpdateEntityResponse(resp)
			case *proto.Response_RemoveEntity:
				c.HandleRemoveEntityResponse(resp)
			case *proto.Response_PlayerRespawn:
				c.HandlePlayerRespawnResponse(resp)
			}
		}
	}()
}

func (c *GameClient) HandleMoveChange(change backend.MoveChange) {
	req := proto.Request{
		Action: &proto.Request_Move{
			Move: &proto.Move{
				Direction: proto.GetProtoDirection(change.Direction),
			},
		},
	}
	c.Stream.Send(&req)
}

// @todo Is this the right way to respond to changes?
func (c *GameClient) HandleAddEntityChange(change backend.AddEntityChange) {
	switch change.Entity.(type) {
	case *backend.Laser:
		laser := change.Entity.(*backend.Laser)
		req := proto.Request{
			Action: &proto.Request_Laser{
				Laser: proto.GetProtoEntity(laser).GetLaser(),
			},
		}
		c.Stream.Send(&req)
	}
}

// HandleInitializeResponse initializes the local player with information
// provided by the server.
func (c *GameClient) HandleInitializeResponse(resp *proto.Response) {
	init := resp.GetInitialize()
	for _, entity := range init.Entities {
		backendEntity := proto.GetBackendEntity(entity)
		if backendEntity == nil {
			// @todo handle
			return
		}
		c.Game.AddEntity(backendEntity)
	}
	c.View.CurrentPlayer = c.CurrentPlayer
	c.View.Paused = false
}

func (c *GameClient) HandleAddEntityResponse(resp *proto.Response) {
	add := resp.GetAddEntity()
	entity := proto.GetBackendEntity(add.Entity)
	if entity == nil {
		// @todo handle
		return
	}
	c.Game.AddEntity(entity)
}

func (c *GameClient) HandleUpdateEntityResponse(resp *proto.Response) {
	update := resp.GetUpdateEntity()
	entity := proto.GetBackendEntity(update.Entity)
	if entity == nil {
		// @todo handle
		return
	}
	c.Game.UpdateEntity(entity)
}

func (c *GameClient) HandleRemoveEntityResponse(resp *proto.Response) {
	remove := resp.GetRemoveEntity()
	id, err := uuid.Parse(remove.Id)
	if err != nil {
		// @todo handle
		return
	}
	c.Game.RemoveEntity(id)
}

func (c *GameClient) HandlePlayerRespawnResponse(resp *proto.Response) {
	respawn := resp.GetPlayerRespawn()
	player := proto.GetBackendPlayer(respawn.Player)
	if player == nil {
		// @todo handle
		return
	}
	c.Game.UpdateEntity(player)
}
