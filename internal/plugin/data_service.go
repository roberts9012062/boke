// internal/plugin/data_service.go
// 主进程只读数据服务（M3.8 插件能力授权：data.read）：
//   经 go-plugin GRPCBroker 注册（AcceptAndServe），授权插件（声明 data.read）在
//   Activate 时收到 brokerID 并 Dial——插件经 sdk.Data(ctx) 查询脱敏数据。
//   解耦：DataProvider 接口由 service 层实现注入（避免 plugin→repository 依赖）。
package plugin

import (
	"context"

	"github.com/roberts9012062/boke/pkg/plugin-sdk/proto"
)

// DataProvider 主进程只读数据查询回调（service 层实现；返回脱敏数据）。
type DataProvider interface {
	// GetUser 查询用户脱敏信息（不存在返回错误——由调用方按场景兜底）。
	GetUser(ctx context.Context, userID int64) (*proto.UserInfo, error)
	// GetPost 查询帖子脱敏信息。
	GetPost(ctx context.Context, postID int64) (*proto.PostInfo, error)
	// GetSettings 查询站点公开设置（白名单键）。
	GetSettings(ctx context.Context) (*proto.SettingsSnapshot, error)
	// GetAIModels 查询可用 AI 模型（脱敏：供应商名 + 模型列表，不含 Key；插件 AI 辅助）。
	GetAIModels(ctx context.Context) (*proto.AIModelList, error)
	// GenerateAI 调用主进程 AI 生成文本（按模型路由供应商；插件 AI 辅助）。
	GenerateAI(ctx context.Context, model string, prompt string, content string) (*proto.GenerateResult, error)
}

// dataServiceServer DataService gRPC 实现（broker 注册时构造）。
type dataServiceServer struct {
	proto.UnimplementedDataServiceServer
	provider DataProvider // 数据查询回调（service 层注入）
}

// GetUser 查询用户脱敏信息。
func (s *dataServiceServer) GetUser(ctx context.Context, req *proto.UserRequest) (*proto.UserInfo, error) {
	if s.provider == nil {
		return &proto.UserInfo{Id: req.GetUserId()}, nil // 未注入：返回空占位（不阻断插件）
	}
	return s.provider.GetUser(ctx, req.GetUserId())
}

// GetPost 查询帖子脱敏信息。
func (s *dataServiceServer) GetPost(ctx context.Context, req *proto.PostRequest) (*proto.PostInfo, error) {
	if s.provider == nil {
		return &proto.PostInfo{Id: req.GetPostId()}, nil
	}
	return s.provider.GetPost(ctx, req.GetPostId())
}

// GetSettings 查询站点公开设置（白名单键）。
func (s *dataServiceServer) GetSettings(ctx context.Context, _ *proto.Empty) (*proto.SettingsSnapshot, error) {
	if s.provider == nil {
		return &proto.SettingsSnapshot{Values: map[string]string{}}, nil
	}
	return s.provider.GetSettings(ctx)
}

// GetAIModels 查询可用 AI 模型（脱敏；未注入返回空列表）。
func (s *dataServiceServer) GetAIModels(ctx context.Context, _ *proto.Empty) (*proto.AIModelList, error) {
	if s.provider == nil {
		return &proto.AIModelList{Models: []*proto.AIModel{}}, nil
	}
	return s.provider.GetAIModels(ctx)
}

// GenerateAI 调用主进程 AI 生成文本（未注入返回空文本）。
func (s *dataServiceServer) GenerateAI(ctx context.Context, req *proto.GenerateRequest) (*proto.GenerateResult, error) {
	if s.provider == nil {
		return &proto.GenerateResult{Text: ""}, nil
	}
	return s.provider.GenerateAI(ctx, req.GetModel(), req.GetPrompt(), req.GetContent())
}
