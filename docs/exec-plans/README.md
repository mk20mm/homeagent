# 执行计划（Exec Plans）

> 计划是**一等公民**：带状态与决策日志，版本化进仓库。智能体不依赖外部上下文即可推进。
> 对齐 AI-STD-006（仓库 Harness）。

## 状态约定

| 标记 | 含义           |
| ---- | -------------- |
| ⏳   | 未开始         |
| 🚧   | 进行中         |
| ✅   | 已完成         |
| ❌   | 放弃（附理由） |

## 目录

```
exec-plans/
├── README.md          # 本文件
├── active/            # 正在推进的计划
└── completed/         # 已归档（保留决策日志）
```

## 活跃计划

| 计划                                       | 状态           | 说明                                                                         |
| ------------------------------------------ | -------------- | ---------------------------------------------------------------------------- |
| [active/skeleton.md](active/skeleton.md)   | 🚧 C 阶段      | 三阶段骨架：A 后端生成链路 ✅ / B 前端工作区 ✅ / C 运行时联通 🚧            |
| [active/c-runtime.md](active/c-runtime.md) | 🚧 P2 待开始   | C 阶段细化：P0 关键路径 ✅ 9/9 / P1 工具集+JWT ✅ 10/11 / P2 评测+联调 ⏳     |
| [active/travel-skeleton.md](active/travel-skeleton.md) | 🚧 Phase D | M6 家庭出行模块骨架与底部 Tab 栏 5 入口对齐 |
| [active/agent-dispatch-center.md](active/agent-dispatch-center.md) | 🚧 Phase D | Agent 调度中心架构、状态机、多模型路由与异常容灾体系（彻底解决一直在思考假死） |
| [active/tool-perms-config.md](active/tool-perms-config.md) | ⏳ 下期 | 工具权限可配置化：admin 后台 + 用户自助，权限矩阵出库解耦（不再硬编码种子） |

## 写作约定

- 每份计划含：目标、任务清单（带状态）、**决策日志**（为什么这么选，含被否的方案）、验收标准。
- 完成后整体移入 `completed/`，不移除（决策历史比代码更难重建）。
- 新增计划先在 `active/` 建文件，再在此登记一行。
