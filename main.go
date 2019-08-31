package main

import ("fmt"
"gomaspri"
)



config := gomaspri.ReadConfig("./config.toml")
fmt.println(config)
