variable "beta" {
  type    = bool
  default = false
}
variable "region" {
  type    = string
  default = "us-west-2"
}
variable "k8s_version" {
  type    = string
  default = "1.24"
}
variable "beta_endpoint" {
  type    = string
  default = "https://api.beta.us-west-2.wesley.amazonaws.com"
}
