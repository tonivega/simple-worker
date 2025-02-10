output "ecr_repository_url" {
  description = "URL del repositorio ECR"
  value       = aws_ecr_repository.ecr_repository.repository_url
}
