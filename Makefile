.PHONY: all build test lint generate gen-ent gen-api gen-web migrate run tidy vet lint-web typecheck-web test-web docs-check

GO ?= go

all: tidy generate build

## 依赖整理
tidy:
	$(GO) mod tidy

## 构建
build:
	$(GO) build -o bin/homeagent ./cmd/homeagent

## 运行
run:
	$(GO) run ./cmd/homeagent

## 全部代码生成（ent + OpenAPI + 前端类型）
generate: gen-ent gen-api gen-web

## ent 代码生成（schema 变更后执行）
gen-ent:
	$(GO) generate ./internal/store/...

## OpenAPI → Go server 桩 + client（契约变更后执行）
gen-api:
	cd internal/openapi && $(GO) generate

## 前端类型生成（openapi.yaml → TS schema，web + admin）
gen-web:
	pnpm -r run gen-api

## 前端静态检查（ESLint + Stylelint）
lint-web:
	pnpm run lint

## 前端类型检查
typecheck-web:
	pnpm -r run typecheck

## 前端单测（Vitest + MSW）
test-web:
	pnpm -r run test

## 数据库迁移
migrate:
	$(GO) run ./cmd/homeagent --migrate

## 测试（表驱动单测；本机无 gcc 不支持 -race，CI 环境可加 CGO_ENABLED=1）
test:
	$(GO) test ./... -count=1

## API 测试
test-api:
	$(GO) test ./internal/api/... -count=1

## Agent 回放测试（不真调供应商）
test-agent:
	$(GO) test ./internal/agent/... -count=1

## 评测套件（AI-STD-005）
eval:
	$(GO) test ./evals/... -count=1

## 文档一致性与合规自检（链接 / FR 覆盖 / 图表成对 / 计划 JSON / AI-STD 标注 / prettier）
docs-check:
	node scripts/docs-check.mjs

## 静态检查
vet:
	$(GO) vet ./...

## lint（需 golangci-lint）
lint:
	golangci-lint run ./...

## 格式化
fmt:
	$(GO) fmt ./...
