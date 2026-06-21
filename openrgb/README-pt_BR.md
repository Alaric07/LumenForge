# Servidor de Destino OpenRGB Legado (Herdado)

> [!IMPORTANT]
> Este documento descreve a direção secundária herdada, na qual o LumenForge expõe dispositivos nativos compatíveis para um cliente OpenRGB. Ele **não** descreve a importação de dispositivos OpenRGB para o LumenForge. Para o fluxo principal, consulte [Importação de Dispositivos OpenRGB](../docs/openrgb-import.md).

Com o lançamento da versão 0.6.2 do LumenForge ou o commit mais recente, o LumenForge suporta comunicação nativa de Cliente/Servidor OpenRGB.

Isso resolve efetivamente qualquer problema de comunicação de dispositivo que ocorre quando dois programas tentam se comunicar com o mesmo dispositivo.

Com essa implementação, você pode usar o OpenRGB para seus efeitos RGB e o LumenForge para monitoramento de temperatura, controle de ventiladores, controle de bomba, controle de LCD e tudo mais que o programa oferece.

## Como configurar
### Passo 1
```json
{
  "enableOpenRGBTargetServer": true,
  "openRGBPort": 6743
}
```

- `enableOpenRGBTargetServer` Isso habilitará o listener TCP
- `openRGBPort` Porta TCP para escutar. Esta é a porta do servidor nativo OpenRGB + 1

### Passo 2
```bash
systemctl stop LumenForge
```

### Passo 3
Desabilite o dispositivo no aplicativo OpenRGB, para que não seja processado lá e clique no botão Aplicar. Depois disso, você pode fazer uma nova varredura de dispositivos ou reiniciar o aplicativo OpenRGB.
Para cada dispositivo que você deseja integrar, você terá que desabilitá-lo no OpenRGB.

![Dispositivo desativado no OpenRGB para o servidor de destino herdado](../static/img/openrgb-device.png)

### Passo 4
```bash
systemctl start LumenForge
```

### Passo 5
- Ative a Integração OpenRGB

![Opção de integração OpenRGB herdada no LumenForge](../static/img/openrgb.png)

### Passo 6
No OpenRGB, clique na aba Cliente e conecte à porta 6743.

![Cliente OpenRGB conectado ao servidor de destino herdado do LumenForge](../static/img/openrgb-client.png)

## Dispositivos suportados
| Dispositivo            |
|------------------------|
| iCUE LINK System Hub   |
| iCUE COMMANDER Core    |
| iCUE COMMANDER Core XT |
| iCUE COMMANDER DUO     |
| ELITE AIOs             |
| PLATINUM AIOs          |
| HYDRO AIOs             |
| Memória                |
| MM700                  |
| MM800                  |

À medida que novas versões são lançadas, mais dispositivos serão adicionados à integração.
