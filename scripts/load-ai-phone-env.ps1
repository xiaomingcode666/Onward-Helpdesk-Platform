Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$requiredNames = @(
    'AI_PHONE_TASK_DATABASE_URL',
    'HELPDESK_GO_URL',
    'AI_PHONE_LINE_TENANTS',
    'AI_PHONE_SERVICE_TOKEN',
    'AI_PHONE_SERVICE_ACCOUNT_ID'
)

foreach ($name in $requiredNames) {
    $value = [Environment]::GetEnvironmentVariable($name, 'User')
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "Windows user environment variable '$name' is not configured."
    }
    Set-Item -Path "Env:$name" -Value $value
}

$optionalNames = @('TELNYX_API_KEY', 'TELNYX_PUBLIC_KEY', 'TELNYX_NUMBER_LINES', 'TELNYX_TTS_VOICES', 'TELNYX_WEBHOOK_TOKEN')
foreach ($name in $optionalNames) {
    $value = [Environment]::GetEnvironmentVariable($name, 'User')
    if (-not [string]::IsNullOrWhiteSpace($value)) {
        Set-Item -Path "Env:$name" -Value $value
    }
}

Write-Output 'AI phone user environment loaded into the current PowerShell process.'
Write-Output "AI_PHONE_TASK_DATABASE_URL: $([bool]$env:AI_PHONE_TASK_DATABASE_URL)"
Write-Output "HELPDESK_GO_URL: $env:HELPDESK_GO_URL"
Write-Output "AI_PHONE_LINE_TENANTS: $env:AI_PHONE_LINE_TENANTS"
Write-Output "AI_PHONE_SERVICE_TOKEN: configured ($($env:AI_PHONE_SERVICE_TOKEN.Length) characters)"
Write-Output "TELNYX_API_KEY: $([bool]$env:TELNYX_API_KEY)"
Write-Output "TELNYX_PUBLIC_KEY: $([bool]$env:TELNYX_PUBLIC_KEY)"
Write-Output "TELNYX_WEBHOOK_TOKEN: $([bool]$env:TELNYX_WEBHOOK_TOKEN)"
Write-Output "TELNYX_NUMBER_LINES: $env:TELNYX_NUMBER_LINES"
