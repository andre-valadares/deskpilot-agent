#Requires -RunAsAdministrator
param(
  [Parameter(Mandatory)][string]$Token,
  [Parameter(Mandatory)][string]$Api
)

$ErrorActionPreference = "Stop"
$BinaryName  = "deskpilot-agent.exe"
$InstallDir  = "$env:ProgramFiles\DeskPilot"
$BinaryPath  = "$InstallDir\$BinaryName"
$TaskName    = "DeskPilotAgent"

Write-Host "Instalando DeskPilot Agent..."

# Criar diretório de instalação
if (-not (Test-Path $InstallDir)) {
  New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

# Baixar binário pré-compilado
$ReleaseUrl = "https://github.com/andre-valadares/deskpilot-agent/releases/latest/download/deskpilot-agent_windows_amd64.zip"
if ($ReleaseUrl) {
  Invoke-WebRequest -Uri $ReleaseUrl -OutFile $BinaryPath
} else {
  # Compilar da fonte como fallback
  $goCmd = Get-Command go -ErrorAction SilentlyContinue
  if (-not $goCmd) {
    Write-Error "Go não encontrado. Instale em https://go.dev/dl/ ou aguarde os binários pré-compilados."
  }
  $ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
  Push-Location $ScriptDir
  go build -o $BinaryPath .
  Pop-Location
}

# Salvar configuração
& $BinaryPath --token=$Token --api=$Api --install

# Registrar no Task Scheduler para rodar na inicialização como SYSTEM
$action  = New-ScheduledTaskAction -Execute $BinaryPath
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit ([TimeSpan]::Zero)
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest

Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null

# Iniciar agora sem aguardar reboot
Start-ScheduledTask -TaskName $TaskName

Write-Host "DeskPilot Agent instalado e em execução. Iniciará automaticamente com o Windows."
