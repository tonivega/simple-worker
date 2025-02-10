variable "repository_name" {
  description = "Nombre del repositorio de Docker en ECR."
  type        = string
}

variable "image_tag_mutability" {
  description = "Configuración de mutabilidad de etiquetas de imágenes (inmutable/mutable)."
  type        = string
}
