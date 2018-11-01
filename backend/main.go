package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
)

const configFile = "config.yaml" // TODO: override with command line argument

type AppConfig struct {
	StaticPath       string `yaml:"static_path"`
	LoginStaticPath  string `yaml:"login_static_path"`
	ConfigPath       string `yaml:"config_path"`
	JWTPrivateKey    string `yaml:"jwt_private_key"`
	DBConfig         `yaml:"db"`
	Port             int    `yaml:"port"`
	Debug            bool   `yaml:"debug"`
	ExternalUsersAPI string `yaml:"external_users_api"`
}

type DBConfig struct {
	User               string `yaml:"user"`
	Password           string `yaml:"password"`
	Host               string `yaml:"host"`
	Port               uint16 `yaml:"port"`
	Database           string `yaml:"dbname"`
	ApplicationName    string `yaml:"app_name"`
	PoolMaxConnections int    `yaml:"pool_max_connections"`
	ReconnectSleep     int    `yaml:"reconnect_sleep"`
}

var config AppConfig = AppConfig{
	StaticPath:      "static",
	LoginStaticPath: "static/login",
	ConfigPath:      "config.json",
	JWTPrivateKey:   "keys/jwt.rsa",
	DBConfig: DBConfig{
		Host:               "localhost",
		Port:               5432,
		ApplicationName:    "summa-ui-stack",
		PoolMaxConnections: 5,
		ReconnectSleep:     10,
	},
	Port:             9000,
	Debug:            false,
	ExternalUsersAPI: "",
}

func main() {
	// byt := []byte(`{"num":6.13,"strs":["a","b"]}`)
	// var dat map[string]interface{}
	// if err := json.Unmarshal(byt, &dat); err != nil {
	// 	panic(err)
	// }
	// fmt.Println(dat)
	// return

	fmt.Println("ui-stack-server")

	if fi, err := os.Stat(configFile); err == nil && !fi.IsDir() {
		log.Printf("Reading configuration file %v", configFile)
		data, err := ioutil.ReadFile(configFile)
		if err != nil {
			log.Fatalf("config read error: %v", err)
			// panic(err)
		}
		err = yaml.Unmarshal(data, &config)
		if err != nil {
			log.Fatalf("config parse error: %v", err)
		}
		out, err := yaml.Marshal(config)
		if config.Debug {
			log.Printf("configuration:\n%v--- EOF configuration ---", string(out))
		}
	}

	loadPrivateKey(config.JWTPrivateKey)

	dbGetOrigins()

	defer shutdownDB()

	runServer()
}
