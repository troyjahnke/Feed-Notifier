param(
    [String]$FeedFile,
    [string]$TableName
)

$newEntries = @()
$entries = aws dynamodb scan --table-name $TableName | ConvertFrom-Json | Select-Object -ExpandProperty items |
        ForEach-Object {
            $item = New-Object PSObject
            $item | Add-Member -Type NoteProperty -Name url -Value $_.url.s
            $item | Add-Member -Type NoteProperty -Name name -Value $_.name.s
            $item | Add-Member -Type NoteProperty -Name latest -Value $_.latest.s
            $newEntries += $item
        }
$newEntries | ConvertTo-Json | Out-File -Path "feeds.json"