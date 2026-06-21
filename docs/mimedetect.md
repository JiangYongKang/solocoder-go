# MIME 类型检测器模块

## 1. 模块功能概述

MIME 类型检测器模块提供了一套完整的文件类型识别解决方案，支持多种检测方式和自定义扩展。主要功能包括：

### 1.1 基于文件魔术字的类型识别
读取文件头部若干字节（默认 512 字节），与内置的魔术字签名库进行比对，根据匹配到的签名返回对应的 MIME 类型。支持两种匹配模式：
- **固定偏移量匹配**：魔术字出现在文件的固定位置
- **可变偏移量匹配**：魔术字可能出现在文件开头的任意位置（从指定偏移量开始搜索）

未匹配到任何签名时返回通用二进制类型 `application/octet-stream`。

### 1.2 文件扩展名与类型的双向映射
维护两套映射表：
- **扩展名到 MIME 类型**：支持通过文件扩展名查询默认 MIME 类型
- **MIME 类型到扩展名**：支持通过 MIME 类型查询推荐的文件扩展名

查询不到时返回空字符串。

### 1.3 未知类型的自定义注册
允许调用方在运行时动态注册：
- 自定义魔术字签名
- 自定义扩展名映射
- 自定义 MIME 类型描述

自定义注册的条目可与内置条目共存，当自定义条目与内置条目冲突时，自定义条目优先。

### 1.4 字节流的类型嗅探
直接对内存中的字节数组进行类型嗅探，无需从文件读取。嗅探逻辑与文件魔术字检测完全一致，适用于网络传输中接收到的数据块的实时类型判断。

## 2. 核心结构体职责

### 2.1 `MagicSignature`
魔术字签名结构体，定义了一种文件类型的识别特征。

| 字段 | 类型 | 说明 |
|------|------|------|
| `MagicBytes` | `[]byte` | 魔术字字节序列 |
| `Offset` | `int` | 偏移量，表示从文件的第几个字节开始匹配 |
| `Mode` | `OffsetMode` | 匹配模式（固定偏移 / 可变偏移） |
| `MIMEType` | `string` | 匹配成功时返回的 MIME 类型 |

### 2.2 `OffsetMode`
偏移量匹配模式枚举。

| 常量 | 值 | 说明 |
|------|----|------|
| `OffsetFixed` | 0 | 固定偏移量匹配：魔术字必须出现在 `Offset` 指定的精确位置 |
| `OffsetVariable` | 1 | 可变偏移量匹配：从 `Offset` 位置开始向后搜索，只要在数据范围内找到魔术字即匹配成功 |

### 2.3 `MIMETypeInfo`
MIME 类型详细信息结构体。

| 字段 | 类型 | 说明 |
|------|------|------|
| `MIMEType` | `string` | MIME 类型标识符 |
| `Description` | `string` | 类型的人类可读描述 |

### 2.4 `Detector`
MIME 类型检测器主结构体，采用读写锁保证并发安全。

内部维护以下数据：
- 内置魔术字签名库
- 自定义魔术字签名库
- 内置扩展名 ↔ MIME 类型双向映射
- 自定义扩展名 ↔ MIME 类型双向映射
- 内置 MIME 类型信息库
- 自定义 MIME 类型信息库

## 3. 魔术字签名匹配规则

### 3.1 匹配优先级
1. 先匹配自定义签名库（按注册时间倒序，后注册的优先）
2. 再匹配内置签名库（按定义顺序）

### 3.2 固定偏移匹配规则
```
匹配条件：
- 数据长度 >= 魔术字长度
- Offset >= 0
- Offset + 魔术字长度 <= 数据长度
- data[Offset : Offset+len(MagicBytes)] == MagicBytes
```

### 3.3 可变偏移匹配规则
```
搜索范围：从 Offset 到 (数据长度 - 魔术字长度)
匹配条件：
- 数据长度 >= 魔术字长度
- Offset <= (数据长度 - 魔术字长度)
- 存在 i ∈ [Offset, 数据长度 - 魔术字长度]
  使得 data[i : i+len(MagicBytes)] == MagicBytes
```

### 3.4 内置签名库（部分示例）

| 文件类型 | 魔术字 | 偏移 | 模式 | MIME 类型 |
|----------|--------|------|------|-----------|
| PNG | `89 50 4E 47 0D 0A 1A 0A` | 0 | 固定 | `image/png` |
| JPEG | `FF D8 FF` | 0 | 固定 | `image/jpeg` |
| GIF87a | `47 49 46 38 37 61` | 0 | 固定 | `image/gif` |
| PDF | `25 50 44 46 2D` | 0 | 固定 | `application/pdf` |
| ZIP | `50 4B 03 04` | 0 | 固定 | `application/zip` |
| GZIP | `1F 8B 08` | 0 | 固定 | `application/gzip` |
| TAR | `75 73 74 61 72` | 257 | 固定 | `application/x-tar` |
| MP4 | `66 74 79 70 49 53 4F 4D` | 4 | 固定 | `video/mp4` |
| HTML | `3C 68 74 6D 6C` | 0 | 可变 | `text/html` |
| HTML(DOCTYPE) | `3C 21 44 4F 43 54 59 50 45 20 68 74 6D 6C` | 0 | 可变 | `text/html` |
| UTF-8 BOM | `EF BB BF` | 0 | 固定 | `text/plain; charset=utf-8` |
| ELF | `7F 45 4C 46` | 0 | 固定 | `application/x-executable` |

## 4. 扩展名与 MIME 类型映射关系

### 4.1 内置扩展名映射（部分示例）

| 扩展名 | MIME 类型 |
|--------|-----------|
| `png` | `image/png` |
| `jpg`, `jpeg` | `image/jpeg` |
| `gif` | `image/gif` |
| `pdf` | `application/pdf` |
| `zip` | `application/zip` |
| `gz`, `gzip` | `application/gzip` |
| `tar` | `application/x-tar` |
| `html`, `htm` | `text/html` |
| `txt` | `text/plain` |
| `json` | `application/json` |
| `xml` | `application/xml` |
| `mp3` | `audio/mpeg` |
| `mp4` | `video/mp4` |
| `bin` | `application/octet-stream` |

### 4.2 内置 MIME 类型到扩展名映射（部分示例）

| MIME 类型 | 推荐扩展名 |
|-----------|-----------|
| `image/png` | `png` |
| `image/jpeg` | `jpg` |
| `application/pdf` | `pdf` |
| `application/zip` | `zip` |
| `text/html` | `html` |
| `application/json` | `json` |
| `application/octet-stream` | `bin` |

### 4.3 映射查询规则
1. 扩展名查询时自动归一化：去除前导点号、转为小写
2. MIME 类型查询时自动转为小写
3. 自定义映射优先于内置映射
4. 查询不到返回空字符串

## 5. 主要 API 接口

### 5.1 `NewDetector() *Detector`
创建一个新的 MIME 类型检测器实例，初始化所有内置数据。

### 5.2 `DetectFromFile(path string) (string, error)`
从文件路径检测 MIME 类型。

### 5.3 `DetectFromBytes(data []byte) string`
从字节数组检测 MIME 类型（字节流嗅探）。

### 5.4 `GetMIMETypeByExtension(ext string) string`
通过扩展名查询默认 MIME 类型。

### 5.5 `GetExtensionByMIMEType(mimeType string) string`
通过 MIME 类型查询推荐扩展名。

### 5.6 `GetMIMETypeInfo(mimeType string) (MIMETypeInfo, bool)`
查询 MIME 类型的详细信息。

### 5.7 `RegisterMagicSignature(sig MagicSignature) error`
注册自定义魔术字签名。

### 5.8 `RegisterExtension(ext, mimeType string) error`
注册自定义扩展名映射。

### 5.9 `RegisterMIMETypeInfo(info MIMETypeInfo, defaultExt string) error`
注册自定义 MIME 类型信息，可同时注册默认扩展名。

### 5.10 `ListCustomSignatures() []MagicSignature`
列出所有已注册的自定义签名（返回副本）。

## 6. 错误定义

| 错误 | 说明 |
|------|------|
| `ErrEmptyMIMEType` | MIME 类型不能为空 |
| `ErrEmptyMagicBytes` | 魔术字字节不能为空 |
| `ErrEmptyExtension` | 扩展名不能为空 |
| `ErrInvalidOffset` | 偏移量不能为负数 |
| `ErrNilMagicSignature` | 魔术字签名不能为 nil |

## 7. 使用示例

### 7.1 基本使用
```go
package main

import (
	"fmt"
	"solocoder-go/internal/mimedetect"
)

func main() {
	detector := mimedetect.NewDetector()

	// 从字节数组检测
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mime := detector.DetectFromBytes(pngData)
	fmt.Println("PNG MIME:", mime) // 输出: image/png

	// 从文件检测
	mime, err := detector.DetectFromFile("document.pdf")
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("PDF MIME:", mime) // 输出: application/pdf
	}

	// 通过扩展名查询
	mime = detector.GetMIMETypeByExtension(".jpg")
	fmt.Println("JPG MIME:", mime) // 输出: image/jpeg

	// 通过 MIME 类型查询扩展名
	ext := detector.GetExtensionByMIMEType("application/pdf")
	fmt.Println("PDF ext:", ext) // 输出: pdf
}
```

### 7.2 自定义类型注册
```go
// 注册自定义魔术字签名
customSig := mimedetect.MagicSignature{
    MagicBytes: []byte{0xDE, 0xAD, 0xBE, 0xEF},
    Offset:     0,
    Mode:       mimedetect.OffsetFixed,
    MIMEType:   "application/x-myformat",
}
err := detector.RegisterMagicSignature(customSig)
if err != nil {
    // 处理错误
}

// 注册自定义扩展名映射
err = detector.RegisterExtension("myfmt", "application/x-myformat")

// 注册自定义 MIME 类型信息
info := mimedetect.MIMETypeInfo{
    MIMEType:    "application/x-myformat",
    Description: "My Custom File Format",
}
err = detector.RegisterMIMETypeInfo(info, "myfmt")

// 测试检测
data := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02}
mime := detector.DetectFromBytes(data)
fmt.Println(mime) // 输出: application/x-myformat
```

### 7.3 可变偏移匹配示例
```go
// 注册可变偏移匹配的签名
htmlSig := mimedetect.MagicSignature{
    MagicBytes: []byte("<html"),
    Offset:     0,
    Mode:       mimedetect.OffsetVariable,
    MIMEType:   "text/html",
}
detector.RegisterMagicSignature(htmlSig)

// 即使 HTML 标签前有空白字符也能匹配
data := []byte("   <html><head>...</head></html>")
mime := detector.DetectFromBytes(data)
fmt.Println(mime) // 输出: text/html
```

### 7.4 网络数据实时嗅探
```go
// 假设从网络接收到数据块
func handleNetworkData(data []byte) {
    detector := mimedetect.NewDetector()
    
    // 直接对网络数据进行类型嗅探
    mimeType := detector.DetectFromBytes(data)
    
    switch mimeType {
    case "image/png", "image/jpeg", "image/gif":
        handleImage(data, mimeType)
    case "application/pdf":
        handlePDF(data)
    case "application/zip":
        handleArchive(data)
    default:
        handleGeneric(data, mimeType)
    }
}
```

## 8. 常量定义

| 常量 | 值 | 说明 |
|------|----|------|
| `DefaultMaxReadBytes` | 512 | 默认从文件读取的最大字节数 |
| `OctetStream` | `application/octet-stream` | 未知类型的默认 MIME 类型 |

## 9. 并发安全

`Detector` 结构体内部使用 `sync.RWMutex` 保证并发安全：
- 所有读取操作（检测、查询）使用读锁
- 所有写入操作（注册）使用写锁
- 支持高并发的检测和查询操作
- 注册操作会阻塞所有读取操作，建议在初始化阶段完成注册

## 10. 测试覆盖

单元测试覆盖以下场景：
- 所有内置文件类型的魔术字检测
- 固定偏移和可变偏移匹配
- 扩展名和 MIME 类型的双向查询
- 自定义注册及优先级验证
- 空数据、短数据、未知数据等边界情况
- 并发检测和并发注册
- 完整工作流测试
- 错误处理分支测试

运行测试：
```bash
go test ./internal/mimedetect/ -v
```
