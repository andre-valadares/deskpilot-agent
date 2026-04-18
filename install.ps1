#Requires -RunAsAdministrator
param(
  [Parameter(Mandatory)][string]$Token,
  [Parameter(Mandatory)][string]$Api,
  [switch]$FileLog
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

# Detectar arquitetura
$Arch = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$ZipUrl  = "https://github.com/andre-valadares/deskpilot-agent/releases/latest/download/deskpilot-agent_windows_$Arch.zip"
$TmpZip  = "$env:TEMP\deskpilot-agent.zip"
$TmpDir  = "$env:TEMP\deskpilot-agent-extract"

Write-Host "Baixando binário ($Arch)..."
Invoke-WebRequest -Uri $ZipUrl -OutFile $TmpZip

if (Test-Path $TmpDir) { Remove-Item $TmpDir -Recurse -Force }
Expand-Archive -Path $TmpZip -DestinationPath $TmpDir
Copy-Item "$TmpDir\deskpilot-agent.exe" -Destination $BinaryPath -Force
Remove-Item $TmpZip, $TmpDir -Recurse -Force

# Salvar configuração
$installArgs = @("--token=$Token", "--api=$Api", "--install")
if ($FileLog) { $installArgs += "--debug" }
& $BinaryPath @installArgs

# Regra de firewall — permite receber WoL (UDP porta 9) apenas para este binário
Remove-NetFirewallRule -DisplayName "DeskPilot Agent" -ErrorAction SilentlyContinue
New-NetFirewallRule -DisplayName "DeskPilot Agent" -Direction Inbound -Program $BinaryPath -Protocol UDP -LocalPort 9 -Action Allow | Out-Null

# Registrar no Task Scheduler para rodar na inicialização como SYSTEM
$action  = New-ScheduledTaskAction -Execute $BinaryPath
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit ([TimeSpan]::Zero)
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest

Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null

# Iniciar agora sem aguardar reboot
Start-ScheduledTask -TaskName $TaskName

Write-Host "DeskPilot Agent instalado e em execução. Iniciará automaticamente com o Windows."
