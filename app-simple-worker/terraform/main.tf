# -------------------------------------------------
# ----------------- Module ECR --------------------
# -------------------------------------------------
# module "module_ecr" {
#   source               = "./modules/module_ecr"
#   repository_name      = var.repository_name
#   image_tag_mutability = var.image_tag_mutability
# }

# -------------------------------------------------
# ----------------- Module EC2 --------------------
# -------------------------------------------------
module "module_ec2" {
  source               = "./modules/module_ec2"

  subnet_id = "subnet-03f5b0dc5550de2c4"
  security_group_id = "sg-0cfa3c9505ce28e9a"
}
