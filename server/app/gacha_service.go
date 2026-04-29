package app

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"log/slog"
	"math/rand"
	"os"

	"rpc-server/internal/rpc"
	pb "rpc-server/pb/app"
)

//go:embed operators.json
var operatorsJSON []byte

type operator struct {
	Name   string `json:"name"`
	Rarity int32  `json:"rarity"`
}

var poolByRarity map[int32][]string

func init() {
	var ops []operator
	if err := json.Unmarshal(operatorsJSON, &ops); err != nil {
		panic("failed to parse operators.json: " + err.Error())
	}
	poolByRarity = make(map[int32][]string)
	for _, op := range ops {
		poolByRarity[op.Rarity] = append(poolByRarity[op.Rarity], op.Name)
	}
}

var rarityProb = []struct {
	stars int32
	prob  float64
}{
	{6, 0.02},
	{5, 0.08},
	{4, 0.50},
	{3, 0.40},
}

type GachaService struct {
	db       *sql.DB
	instance string
}

func NewGachaService(database *sql.DB) *GachaService {
	host, _ := os.Hostname()
	return &GachaService{db: database, instance: host}
}

func (s *GachaService) SetInstance(port string) {
	s.instance = port
}

func (s *GachaService) Draw(ctx context.Context, req *pb.DrawRequest) (*pb.DrawResponse, error) {
	count := req.Count
	if count < 1 {
		count = 1
	}
	if count > 10 {
		count = 10
	}

	cards := make([]*pb.Card, 0, count)
	for i := int32(0); i < count; i++ {
		card := drawOne()
		s.db.ExecContext(ctx, "INSERT INTO cards (user_id, name, stars) VALUES ($1, $2, $3)", req.UserId, card.Name, card.Stars)
		cards = append(cards, card)
	}
	slog.Info("draw completed", "user_id", req.UserId, "count", count, "instance", s.instance)
	return &pb.DrawResponse{Cards: cards, Instance: s.instance}, nil
}

func drawOne() *pb.Card {
	r := rand.Float64()
	cumulative := 0.0
	var stars int32 = 3
	for _, tier := range rarityProb {
		cumulative += tier.prob
		if r < cumulative {
			stars = tier.stars
			break
		}
	}

	names := poolByRarity[stars]
	name := names[rand.Intn(len(names))]
	return &pb.Card{Name: name, Stars: stars}
}

var _ rpc.GachaServiceHandler = (*GachaService)(nil)
