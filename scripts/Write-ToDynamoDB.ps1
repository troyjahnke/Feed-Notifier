param(
    [String]$FeedFile,
    [string]$TableName
)

$dynamoDBOperations = @()
Get-Content -Path $FeedFile | ConvertFrom-Json | ForEach-Object{
    $dynamoDBOperations += @{
        "PutRequest"=@{
            "Item"=@{
                "url" = @{
                    "S" = $_.url
                };
                "name" = @{
                    "S" = $_.name
                };
                "latest" = @{
                    "S" = $_.latest
                }
            }
        }
    }
}
$dynamoDBOperations = @{
    $TableName=$dynamoDBOperations;
}
$dynamoDBOperations = $dynamoDBOperations | ConvertTo-Json -Depth 5
Write-Host "Writing $dynamoDBOperations..."
aws dynamodb batch-write-item --request-items "$dynamoDBOperations"