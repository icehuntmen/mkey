# Mkey ID Generator (Enhanced)

[![Go Reference](https://pkg.go.dev/badge/github.com/icehuntmen/mkey.svg)](https://pkg.go.dev/github.com/icehuntmen/mkey)

Улучшенная реализация генератора Mkay ID для распределенных систем с дополнительными функциями.

## Особенности

- Генерация уникальных, сортируемых по времени ID
- Поддержка кастомных параметров (биты для node/step, эпоха)
- Пакетная генерация ID (`GenerateBatch`)
- Поддержка различных кодировок:
    - Base32 (кастомный алфавит)
    - Base58
    - URL-safe Base64
    - Base36
    - Base2 (бинарный формат)
- Методы для извлечения компонентов ID (timestamp, node ID, sequence)
- Потокобезопасность
- Поддержка JSON marshaling/unmarshaling

## Установка

```bash
go get github.com/icehuntmen/mkey
```

## Использование

### Базовый пример

```go
package main

import (
	"fmt"
	"github.com/icehuntmen/mkey"
)

func main() {
	// Создаем ноду с node ID = 1
	node, err := mkey.NewNode(1)
	if err != nil {
		panic(err)
	}

	// Генерируем ID
	id := node.Generate()

	fmt.Println("ID:", id)
	fmt.Println("Base32:", id.Base32())
	fmt.Println("Timestamp:", id.Timestamp(node))
}
```

### Расширенная конфигурация

```go
cfg := &mkey.Config{
	Epoch:    mkey.DefaultEpoch, // Кастомная эпоха
	NodeBits: 12,                // 12 бит для node ID
	StepBits: 10,                // 10 бит для sequence
	Node:     42,                // Node ID
}

node, err := mkey.NewNodeWithConfig(cfg)
```

### Пакетная генерация

```go
// Генерация 100 ID за один вызов
ids, err := node.GenerateBatch(100)
if err != nil {
	panic(err)
}
```

## Формат ID

Стандартный формат Snowflake ID (64 бита):

```
| 41 бит timestamp | 10 бит node ID | 12 бит sequence |
```

Но вы можете настроить распределение битов через `Config`.

## Методы ID

Основные методы для работы с ID:

```go
id := node.Generate()

// Получение компонентов
timestamp := id.Timestamp(node) // time.Time
nodeID := id.NodeID(node)      // int64
sequence := id.Step(node)      // int64

// Кодировки
str := id.String()     // Десятичная строка
b32 := id.Base32()     // Base32
b58 := id.Base58()     // Base58
b64 := id.Base64()     // URL-safe Base64
bin := id.Base2()      // Бинарное представление

// Парсинг
parsedID, err := mkey.ParseString("1234567890")
```

## Ограничения

1. Максимальное значение `NodeBits + StepBits` = 22 (так как 41 бит зарезервирован под timestamp)
2. Node ID должен быть в диапазоне [0, 2^NodeBits-1]
3. Количество ID, генерируемых за мс, ограничено `StepBits`

## Лицензия

MIT License. См. файл [LICENSE](LICENSE).