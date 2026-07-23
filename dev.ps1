# dev.ps1 - Start all services locally in parallel
# Usage: .\dev.ps1

Write-Host "Starting Despliegue Facilito services..." -ForegroundColor Cyan

# Start backend
$backend = Start-Process -NoNewWindow -PassThru -FilePath "go" -ArgumentList "run", "./cmd/server" -WorkingDirectory "$PSScriptRoot\apps\backend"
Write-Host "[Backend]  PID $($backend.Id) - http://localhost:8080" -ForegroundColor Green

# Start frontend (React dashboard)
$frontend = Start-Process -NoNewWindow -PassThru -FilePath "pnpm" -ArgumentList "dev" -WorkingDirectory "$PSScriptRoot\apps\frontend"
Write-Host "[Frontend] PID $($frontend.Id) - http://localhost:5173" -ForegroundColor Green

# Start landing (Astro)
$landing = Start-Process -NoNewWindow -PassThru -FilePath "pnpm" -ArgumentList "dev" -WorkingDirectory "$PSScriptRoot\apps\landing"
Write-Host "[Landing]  PID $($landing.Id) - http://localhost:4321" -ForegroundColor Green

Write-Host ""
Write-Host "All services started. Press Ctrl+C to stop." -ForegroundColor Yellow

# Wait for any process to exit, then stop all
try {
    Wait-Process -Id $backend.Id, $frontend.Id, $landing.Id -ErrorAction SilentlyContinue
} finally {
    Write-Host "Stopping all services..." -ForegroundColor Red
    Stop-Process -Id $backend.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $frontend.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $landing.Id -ErrorAction SilentlyContinue
}
