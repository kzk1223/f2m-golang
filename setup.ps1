$dirs = @(
  "cmd\server",
  "cmd\cgi",
  "internal\app",
  "internal\config",
  "internal\form",
  "internal\mailer",
  "internal\template",
  "internal\security",
  "internal\charset",
  "internal\storage",
  "internal\logger",
  "templates",
  "forms",
  "storage\tmp",
  "storage\csv",
  "storage\logs",
  "public"
)

$dirs | ForEach-Object {
    New-Item -ItemType Directory -Force -Path $_ | Out-Null
}