package app

import (
	"context"
	"database/sql"
	"fmt"

	"rpc-server/internal/db"
	"rpc-server/internal/rpc"
	pb "rpc-server/pb/app"
)

type UserService struct {
	db *sql.DB
}

func NewUserService(database *sql.DB) *UserService {
	return &UserService{db: database}
}

func (s *UserService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	hash := db.HashPassword(req.Password)
	result, err := s.db.ExecContext(ctx, "INSERT INTO users (name, password_hash) VALUES (?, ?)", req.Name, hash)
	if err != nil {
		return &pb.RegisterResponse{Message: "注册失败: 用户名已被占用"}, nil
	}
	id, _ := result.LastInsertId()
	return &pb.RegisterResponse{
		UserId:  id,
		Message: fmt.Sprintf("注册成功! user_id=%d", id),
	}, nil
}

func (s *UserService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	hash := db.HashPassword(req.Password)
	var id int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM users WHERE name = ? AND password_hash = ?", req.Name, hash).Scan(&id)
	if err != nil {
		return &pb.LoginResponse{Message: "登录失败: 用户名或密码错误"}, nil
	}
	return &pb.LoginResponse{
		UserId:  id,
		Message: fmt.Sprintf("登录成功! 欢迎回来, %s", req.Name),
	}, nil
}

func (s *UserService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	return &pb.LogoutResponse{Message: "已登出"}, nil
}

var _ rpc.UserServiceHandler = (*UserService)(nil)
