// internal/plugin/data_service.go
// 主进程只读数据服务（插件能力授权：data.read）：
//   经 go-plugin MuxBroker 注册（AcceptAndServe，服务名固定 "Plugin"），授权插件
//   （声明 data.read）在 Activate 时收到 brokerID 并 Dial——插件经 sdk.Data(ctx)
//   查询脱敏数据。net/rpc + gob 传输（契约 contract 包），无 gRPC 依赖。
//   解耦：DataProvider 接口由 service 层实现注入（避免 plugin→repository 依赖）。
package plugin

import (
	"context"

	"github.com/roberts9012062/boke/pkg/plugin-sdk/contract"
)

// DataProvider 主进程只读数据查询回调（service 层实现；返回脱敏数据）。
type DataProvider interface {
	// GetUser 查询用户脱敏信息（不存在返回错误——由调用方按场景兜底）。
	GetUser(ctx context.Context, userID int64) (*contract.UserInfo, error)
	// GetPost 查询帖子脱敏信息。
	GetPost(ctx context.Context, postID int64) (*contract.PostInfo, error)
	// GetSettings 查询站点公开设置（白名单键）。
	GetSettings(ctx context.Context) (*contract.SettingsSnapshot, error)
	// GetAIModels 查询可用 AI 模型（脱敏：供应商名 + 模型列表，不含 Key；插件 AI 辅助）。
	GetAIModels(ctx context.Context) (*contract.AIModelList, error)
	// GenerateAI 调用主进程 AI 生成文本（按模型路由供应商；插件 AI 辅助）。
	GenerateAI(ctx context.Context, model string, prompt string, content string) (*contract.GenerateResult, error)
	// GetOpenAPIKeys 查询开放接口 API Key 清单（含明文 Key；浏览器插件联动场景：
	// 插件把 Key 远传给配套浏览器插件，后者凭 X-Api-Key 调用 /api/v1/open/*）。
	GetOpenAPIKeys(ctx context.Context) (*contract.OpenAPIKeyList, error)
}

// dataRPCServer 数据服务 net/rpc 实现（broker 注册时构造；方法即插件调用路径
// "Plugin.XXX"——MuxBroker.AcceptAndServe 的固定注册名约定）。
// 说明：net/rpc 无 ctx 透传，桥接层以 context.Background() 调用 provider——
// 数据查询为毫秒级本地读，无需超时控制（与旧 gRPC 版行为等价）。
type dataRPCServer struct {
	provider DataProvider // 数据查询回调（service 层注入）
}

// GetUser 查询用户脱敏信息。
func (s *dataRPCServer) GetUser(args contract.UserRequest, reply *contract.UserInfo) error {
	if s.provider == nil {
		*reply = contract.UserInfo{ID: args.UserID} // 未注入：返回空占位（不阻断插件）
		return nil
	}
	info, err := s.provider.GetUser(context.Background(), args.UserID)
	if err != nil {
		return err
	}
	*reply = *info
	return nil
}

// GetPost 查询帖子脱敏信息。
func (s *dataRPCServer) GetPost(args contract.PostRequest, reply *contract.PostInfo) error {
	if s.provider == nil {
		*reply = contract.PostInfo{ID: args.PostID}
		return nil
	}
	info, err := s.provider.GetPost(context.Background(), args.PostID)
	if err != nil {
		return err
	}
	*reply = *info
	return nil
}

// GetSettings 查询站点公开设置（白名单键）。
func (s *dataRPCServer) GetSettings(_ contract.Empty, reply *contract.SettingsSnapshot) error {
	if s.provider == nil {
		*reply = contract.SettingsSnapshot{Values: map[string]string{}}
		return nil
	}
	snapshot, err := s.provider.GetSettings(context.Background())
	if err != nil {
		return err
	}
	*reply = *snapshot
	return nil
}

// GetAIModels 查询可用 AI 模型（脱敏；未注入返回空列表）。
func (s *dataRPCServer) GetAIModels(_ contract.Empty, reply *contract.AIModelList) error {
	if s.provider == nil {
		*reply = contract.AIModelList{Models: []contract.AIModel{}}
		return nil
	}
	list, err := s.provider.GetAIModels(context.Background())
	if err != nil {
		return err
	}
	*reply = *list
	return nil
}

// GenerateAI 调用主进程 AI 生成文本（未注入返回空文本）。
func (s *dataRPCServer) GenerateAI(args contract.GenerateRequest, reply *contract.GenerateResult) error {
	if s.provider == nil {
		*reply = contract.GenerateResult{Text: ""}
		return nil
	}
	result, err := s.provider.GenerateAI(context.Background(), args.Model, args.Prompt, args.Content)
	if err != nil {
		return err
	}
	*reply = *result
	return nil
}

// GetOpenAPIKeys 查询开放接口 API Key 清单（未注入返回空列表）。
func (s *dataRPCServer) GetOpenAPIKeys(_ contract.Empty, reply *contract.OpenAPIKeyList) error {
	if s.provider == nil {
		*reply = contract.OpenAPIKeyList{Keys: []contract.OpenAPIKeyInfo{}}
		return nil
	}
	list, err := s.provider.GetOpenAPIKeys(context.Background())
	if err != nil {
		return err
	}
	*reply = *list
	return nil
}
