# 测试指南（Testing Guide）

xsql 项目采用三层测试策略：单元测试、集成测试、端到端测试（E2E）。

## 测试层次

| 层次 | 位置 | Build Tag | 依赖 | 运行速度 |
|------|------|-----------|------|----------|
| 单元测试 | `internal/*/`、`cmd/xsql/` | 无 | 无外部依赖 | 快（秒级） |
| 集成测试 | `tests/integration/` | `integration` | MySQL + PostgreSQL | 中（10-30秒） |
| E2E 测试 | `tests/e2e/` | `e2e` | 无（或可选数据库） | 中（数秒） |

## 快速开始

```bash
# 运行所有单元测试
go test ./...

# 运行集成测试（需要数据库）
go test -tags=integration ./tests/integration/...

# 运行 E2E 测试
go test -tags=e2e ./tests/e2e/...

# 全部测试（带覆盖率）
go test -cover ./...
```

---

## 单元测试

### 定位
- 测试单个函数/方法的逻辑正确性
- 不依赖外部资源（数据库、文件系统、网络）
- 快速、隔离、可重复

### 目录结构
测试文件放在被测代码同目录，命名为 `*_test.go`：
```
internal/config/
  ├── loader.go
  ├── loader_test.go    # 单元测试
  ├── types.go
  └── types_test.go
```

### 最佳实践

#### 1. 表驱动测试
```go
func TestParseDBType(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"mysql", "mysql", "mysql", false},
        {"pg", "pg", "pg", false},
        {"postgres alias", "postgres", "pg", false},
        {"invalid", "oracle", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseDBType(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```

#### 2. 使用 t.Helper()
```go
func assertOK(t *testing.T, resp Response) {
    t.Helper() // 错误定位到调用处
    if !resp.OK {
        t.Errorf("expected ok=true, got false")
    }
}
```

#### 3. 使用 t.TempDir() 处理临时文件
```go
func TestLoadConfig(t *testing.T) {
    tmpDir := t.TempDir() // 测试结束自动清理
    configPath := filepath.Join(tmpDir, "xsql.yaml")
    // ...
}
```

#### 4. 避免全局状态
- 不要修改全局变量
- 使用依赖注入便于 mock

### 覆盖范围
单元测试应覆盖：
- 配置优先级合并（CLI > ENV > Config）
- DSN/URL 解析
- 只读 SQL 判定（允许/拒绝用例）
- 输出序列化（json/yaml/csv/table）
- 错误码映射

---

## 集成测试

### 定位
- 测试多个模块协作
- 依赖真实数据库
- 验证 SQL 执行、只读策略等端到端逻辑

### 目录结构
```
tests/integration/
  ├── cli_test.go      # CLI 基础测试
  ├── db_test.go       # 数据库连接测试
  ├── query_test.go    # 查询相关测试
  └── secret_test.go   # 密钥管理测试
```

### 启动测试数据库
```bash
# 使用 docker-compose
docker-compose up -d

# 等待就绪
docker-compose ps
```

### 运行测试
```bash
# Linux/macOS
XSQL_TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/testdb?parseTime=true" \
XSQL_TEST_PG_DSN="postgres://postgres:postgres@127.0.0.1:5432/testdb?sslmode=disable" \
go test -v -tags=integration ./tests/integration/...

# Windows PowerShell
$env:XSQL_TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/testdb?parseTime=true"
$env:XSQL_TEST_PG_DSN="postgres://postgres:postgres@127.0.0.1:5432/testdb?sslmode=disable"
go test -v -tags=integration ./tests/integration/...
```

### 最佳实践

#### 1. 使用 build tag 隔离
```go
//go:build integration

package integration
```

#### 2. 跳过无环境测试
```go
func TestMySQL_Query(t *testing.T) {
    dsn := os.Getenv("XSQL_TEST_MYSQL_DSN")
    if dsn == "" {
        t.Skip("XSQL_TEST_MYSQL_DSN not set")
    }
    // ...
}
```

#### 3. 使用 TestMain 构建二进制
```go
var testBinary string

func TestMain(m *testing.M) {
    tmpDir, _ := os.MkdirTemp("", "xsql-test")
    defer os.RemoveAll(tmpDir)
    
    testBinary = filepath.Join(tmpDir, "xsql")
    cmd := exec.Command("go", "build", "-o", testBinary, "../../cmd/xsql")
    if out, err := cmd.CombinedOutput(); err != nil {
        panic(string(out))
    }
    
    os.Exit(m.Run())
}
```

#### 4. 清理测试数据
- 使用事务 + 回滚
- 或使用临时表/数据库

### 覆盖范围
- MySQL/PostgreSQL 基本查询
- 多种数据类型处理
- 只读策略拦截（写操作被阻止）
- 连接错误处理
- 各种输出格式

---

## E2E 测试

### 定位
- 黑盒测试：通过 CLI 接口测试整个系统
- 验证用户场景
- 不关心内部实现

### 目录结构
```
tests/e2e/
  ├── e2e_test.go                # 共享 helper 和基础设施
  ├── mcp_test.go                # MCP Server 测试
  ├── output_test.go             # 输出格式测试
  ├── profile_test.go            # profile 命令测试
  ├── proxy_test.go              # proxy 命令测试
  ├── readonly_test.go           # 只读策略测试
  └── ssh_proxy_success_test.go  # SSH 代理成功测试
```

### 运行测试
```bash
# 不需要数据库的测试
go test -tags=e2e ./tests/e2e/...

# 需要数据库的测试（设置 DSN 环境变量）
XSQL_TEST_MYSQL_DSN="..." go test -tags=e2e ./tests/e2e/...
```

### 最佳实践

#### 1. 封装 CLI 调用
```go
func runXSQL(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
    t.Helper()
    cmd := exec.Command(testBinary, args...)
    var outBuf, errBuf bytes.Buffer
    cmd.Stdout = &outBuf
    cmd.Stderr = &errBuf
    
    err := cmd.Run()
    exitCode = 0
    if exitErr, ok := err.(*exec.ExitError); ok {
        exitCode = exitErr.ExitCode()
    }
    return outBuf.String(), errBuf.String(), exitCode
}
```

#### 2. 使用临时配置文件
```go
func createTempConfig(t *testing.T, content string) string {
    t.Helper()
    tmpDir := t.TempDir()
    configPath := filepath.Join(tmpDir, "xsql.yaml")
    os.WriteFile(configPath, []byte(content), 0600)
    return configPath
}
```

#### 3. 验证输出格式
```go
func TestProfile_List_JSON(t *testing.T) {
    config := createTempConfig(t, `profiles:
  dev:
    description: "开发环境"
    db: mysql
`)
    stdout, _, exitCode := runXSQL(t, "profile", "list", "--config", config, "--format", "json")
    
    if exitCode != 0 {
        t.Fatalf("exit code %d", exitCode)
    }
    
    var resp struct {
        OK   bool `json:"ok"`
        Data struct {
            Profiles []struct {
                Name        string `json:"name"`
                Description string `json:"description"`
            } `json:"profiles"`
        } `json:"data"`
    }
    json.Unmarshal([]byte(stdout), &resp)
    
    if !resp.OK {
        t.Error("expected ok=true")
    }
}
```

#### 4. 验证 Table 输出
```go
func TestProfile_List_Table(t *testing.T) {
    // ...
    stdout, _, _ := runXSQL(t, "profile", "list", "--format", "table")
    
    // 验证表头
    if !strings.Contains(stdout, "NAME") {
        t.Error("missing NAME header")
    }
    if !strings.Contains(stdout, "DESCRIPTION") {
        t.Error("missing DESCRIPTION header")
    }
}
```

#### 5. 验证退出码
```go
func TestProfile_Show_NotFound(t *testing.T) {
    config := createTempConfig(t, `profiles: {}`)
    
    _, _, exitCode := runXSQL(t, "profile", "show", "nonexistent", "--config", config)
    
    if exitCode != 2 { // XSQL_CFG_INVALID
        t.Errorf("expected exit code 2, got %d", exitCode)
    }
}
```

### 覆盖范围
- 所有命令的 JSON/YAML/Table/CSV 输出
- 错误场景的退出码
- 密码脱敏
- Unicode/特殊字符处理
- 配置文件不存在等边界情况

---

## 测试命名规范

```
Test<Module>_<Function>_<Scenario>

示例：
- TestConfig_Load_Priority      # 配置加载优先级
- TestQuery_MySQL_ReadOnly      # MySQL 只读查询
- TestProfile_List_JSON         # profile list 的 JSON 输出
- TestProfile_Show_NotFound     # profile show 找不到
```

---

## CI 集成

GitHub Actions 自动运行：

```yaml
# 单元测试（所有 PR）
- run: go test ./...

# 集成测试（需要数据库服务）
- run: go test -tags=integration ./tests/integration/...

# E2E 测试
- run: go test -tags=e2e ./tests/e2e/...
```

详见 `.github/workflows/ci.yml`。

---

## 添加新功能的测试清单

1. **单元测试**：在 `internal/*/` 添加核心逻辑测试
2. **E2E 测试**：在 `tests/e2e/` 验证 CLI 输出（JSON + Table）
3. **集成测试**（如涉及数据库）：在 `tests/integration/` 验证端到端
4. **更新文档**：确保 `docs/cli-spec.md` 的输出示例与实际一致
