// Package openapi 存放由 api/openapi.yaml 生成的接口与类型（勿手改，CONVENTIONS-backend §1）。
package openapi

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config ../../api/oapi-codegen.yaml ../../api/openapi.yaml
