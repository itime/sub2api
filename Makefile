.PHONY: build build-backend build-frontend build-release build-datamanagementd \
	deploy deploy-check test test-backend test-frontend test-frontend-critical \
	test-datamanagementd secret-scan

FRONTEND_CRITICAL_VITEST := \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts

# 一键编译前后端（开发用：不嵌入 UI，不更新 :18080 的 supervisor 进程）
build: build-backend build-frontend
	@echo ""
	@echo "Note: 'make build' does NOT update http://127.0.0.1:18080."
	@echo "      Run 'make deploy' to release to /usr/local/bin/sub2api (supervisor)."

# 编译后端（复用 backend/Makefile，产出 backend/bin/server，无 embed）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 生产本地二进制（embed 前端 -> ./sub2api，不安装、不重启）
build-release:
	@./deploy.sh --build-only

# 本地发布：构建 embed 二进制 + 安装到 /usr/local/bin/sub2api + supervisor 重启 + 健康检查
deploy:
	@./deploy.sh

# 对比 git / dist / 已安装二进制 / 健康检查
deploy-check:
	@./deploy/deploy-check.sh

# 编译 datamanagementd（宿主机数据管理进程）
build-datamanagementd:
	@cd datamanagement && go build -o datamanagementd ./cmd/datamanagementd

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck
	@$(MAKE) test-frontend-critical

test-frontend-critical:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)

test-datamanagementd:
	@cd datamanagement && go test ./...

secret-scan:
	@python3 tools/secret_scan.py
