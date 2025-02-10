resource "aws_instance" "practica_provisioner" {
  ami                         = "ami-0450bfd9dee9bca43" 
  instance_type               = "t2.micro"
  key_name                    = "key-simple-worker"

  associate_public_ip_address = true
  subnet_id                   = var.subnet_id
  vpc_security_group_ids      = [var.security_group_id]

  connection {
    type        = "ssh"
    user        = "root"
    private_key = file("~/.ssh/key-simple-worker.pem") 
    host        = self.public_ip
  }

  tags = {
    Name = "simple-worker"
  }
}

