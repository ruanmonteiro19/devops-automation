#Requires -RunAsAdministrator
param([Parameter(Mandatory=$true)][string]$Binary,[Parameter(Mandatory=$true)][string]$Server,
[string]$HostGroup="",[string]$NetworkZone="default",[string]$CACert="")
$ErrorActionPreference="Stop"
if ($Server -notmatch '^https://' -and $Server -notmatch '^http://(127[.]0[.]0[.]1|localhost):') {throw "Gateway remoto exige HTTPS"}
if (-not (Test-Path -LiteralPath $Binary)){throw "Binario inexistente"}
$secure=Read-Host "Token do agente" -AsSecureString
$ptr=[Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
try {$token=[Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)}
finally {[Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)}
if ($token.Length -lt 24){throw "Token muito curto"}
$folder=Join-Path $env:ProgramData 'Observa Lite'
$exeFolder=Join-Path $env:ProgramFiles 'Observa Lite'
New-Item -ItemType Directory -Path $folder,$exeFolder -Force | Out-Null
Copy-Item -LiteralPath $Binary -Destination (Join-Path $exeFolder 'observa.exe') -Force
$config=@("OBSERVA_AGENT_TOKEN=$token","OBSERVA_SERVER=$Server","OBSERVA_HOST_GROUP=$HostGroup","OBSERVA_NETWORK_ZONE=$NetworkZone")
if ($CACert) {
 Copy-Item $CACert (Join-Path $folder 'ca.pem') -Force
 $config+="OBSERVA_CA_FILE=$(Join-Path $folder 'ca.pem')"
}
[IO.File]::WriteAllLines((Join-Path $folder 'agent.env'),$config,[Text.UTF8Encoding]::new($false))
$token=$null
& icacls.exe $folder /inheritance:r /grant:r "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F" | Out-Null
if ($LASTEXITCODE -ne 0){throw "Falha ACL"}
$launcher=Join-Path $folder 'start-agent.ps1'
$script=@'
Get-Content (Join-Path $PSScriptRoot 'agent.env') | ForEach-Object {
 $p=$_.Split('=',2)
 if($p.Count -eq 2){[Environment]::SetEnvironmentVariable($p[0],$p[1],'Process')}
}
& (Join-Path $env:ProgramFiles 'Observa Lite\observa.exe') agent
'@
[IO.File]::WriteAllText($launcher,$script,[Text.UTF8Encoding]::new($false))
$action=New-ScheduledTaskAction -Execute 'powershell.exe' -Argument ('-NoProfile -NonInteractive -File "' + $launcher + '"')
$trigger=New-ScheduledTaskTrigger -AtStartup
$principal=New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
Register-ScheduledTask -TaskName 'ObservaLiteAgent' -Action $action -Trigger $trigger -Principal $principal -Force | Out-Null
Start-ScheduledTask -TaskName 'ObservaLiteAgent'
Write-Host 'Agente iniciado como tarefa agendada ObservaLiteAgent (nao serviço SCM).'
