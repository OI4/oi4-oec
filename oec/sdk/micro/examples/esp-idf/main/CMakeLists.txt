idf_component_register(SRCS "main.c" "../../sensor/oecm_sensor.c"
    INCLUDE_DIRS "../../sensor"
    PRIV_REQUIRES oecm nvs_flash esp_netif esp_event esp_wifi esp_eth esp_driver_uart
                  esp_app_format esp_hw_support esp_timer)
