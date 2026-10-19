param(
    [string]$InputDevice = $env:AI_PHONE_INPUT_DEVICE,
    [string]$OutputDevice = $env:AI_PHONE_OUTPUT_DEVICE,
    [string]$Language = "zh-CN",
    [switch]$SkipHelpdeskCheck
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\")).Path
$agentRoot = Join-Path $repoRoot "services\ai_phone_agent"
$python = Join-Path $agentRoot ".venv-ai-phone\Scripts\python.exe"

if (-not (Test-Path -LiteralPath $python)) {
    throw "找不到 Python 虚拟环境：$python。请先安装 services\ai_phone_agent 的依赖。"
}

# The Python runner reads this file itself, but the launcher must also read the
# backend URL/token before it can perform the readiness check.
$localEnv = Join-Path $agentRoot ".env.local"
if (Test-Path -LiteralPath $localEnv) {
    foreach ($line in Get-Content -LiteralPath $localEnv -Encoding UTF8) {
        $trimmed = $line.Trim()
        if ($trimmed -and -not $trimmed.StartsWith("#") -and $trimmed.Contains("=")) {
            $parts = $trimmed.Split("=", 2)
            $name = $parts[0].Trim()
            $value = $parts[1].Trim()
            if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) {
                $value = $value.Substring(1, $value.Length - 2)
            }
            if ($name -and [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($name, "Process"))) {
                Set-Item -Path "Env:$name" -Value $value
            }
        }
    }
}

$goUrl = if ([string]::IsNullOrWhiteSpace($env:HELPDESK_GO_URL)) {
    "http://127.0.0.1:8083"
} else {
    $env:HELPDESK_GO_URL.TrimEnd('/')
}

function Test-HelpdeskHealth {
    try {
        Invoke-RestMethod -Uri "$goUrl/api/health" -Method Get -TimeoutSec 3 | Out-Null
        return $true
    } catch {
        return $false
    }
}

if (-not $SkipHelpdeskCheck) {
    if (-not (Test-HelpdeskHealth)) {
        Write-Host "8083 尚未就绪，正在启动 Helpdesk Go 服务……" -ForegroundColor Yellow
        try {
            $goCommand = Get-Command go -ErrorAction Stop
            Start-Process -FilePath $goCommand.Source `
                -ArgumentList @("run", "./cmd/server", "-config", "config/config.yaml") `
                -WorkingDirectory $repoRoot `
                -WindowStyle Normal | Out-Null
        } catch {
            throw "找不到 Go，且 Helpdesk Go 服务未在 $goUrl 就绪。请先安装 Go 或手动启动后重试。"
        }

        $ready = $false
        for ($attempt = 1; $attempt -le 30; $attempt++) {
            Start-Sleep -Seconds 1
            if (Test-HelpdeskHealth) {
                $ready = $true
                break
            }
        }
        if (-not $ready) {
            throw "Helpdesk Go 服务未能在 30 秒内就绪，请查看新打开的 Go 服务窗口。"
        }
    }
}

if ([string]::IsNullOrWhiteSpace($env:AI_PHONE_SERVICE_TOKEN)) {
    throw "缺少 AI_PHONE_SERVICE_TOKEN。请在 services\ai_phone_agent\.env.local 或 Windows 用户环境变量中配置。"
}

$arguments = @(
    "-m", "ai_phone_agent.local_voice_mic",
    "--helpdesk-reply",
    "--hands-free",
    "--language", $Language,
    "--device", "cpu",
    "--compute-type", "int8"
)

$taskStore = $env:AI_PHONE_TASK_STORE
if ([string]::IsNullOrWhiteSpace($taskStore)) {
    $taskStore = "auto"
    if (-not [string]::IsNullOrWhiteSpace($env:AI_PHONE_TASK_DATABASE_URL)) {
        $hasPsycopg = (& $python -c "import importlib.util; print(bool(importlib.util.find_spec('psycopg')))" 2>$null).Trim()
        if ($hasPsycopg -ne "True") {
            $taskStore = "memory"
            Write-Host "当前虚拟环境没有 psycopg，本地语音验收使用内存任务存储；工单仍会真实调用 Go。" -ForegroundColor Yellow
        }
    }
}
if ($taskStore -notin @("auto", "memory", "postgres")) {
    throw "AI_PHONE_TASK_STORE 必须是 auto、memory 或 postgres。"
}
$arguments += @("--task-store", $taskStore)
if (-not [string]::IsNullOrWhiteSpace($InputDevice)) {
    $arguments += @("--input-device", $InputDevice)
}
if (-not [string]::IsNullOrWhiteSpace($OutputDevice)) {
    $arguments += @("--output-device", $OutputDevice)
}

Write-Host "本地 AI 语音客服已启动。现在可以直接对着麦克风说话；停顿后会自动回答。" -ForegroundColor Green
Write-Host "按 Ctrl+C 结束服务。" -ForegroundColor DarkGray
Push-Location $agentRoot
try {
    & $python @arguments
    if ($LASTEXITCODE -ne 0) {
        Write-Host "本地 AI 语音客服退出，代码：$LASTEXITCODE" -ForegroundColor Yellow
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}
