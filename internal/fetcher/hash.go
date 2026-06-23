package fetcher

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
)

// GetIconHash 下载指定 URL 的图片并计算其特征 Hash 值
// 这里为了不引入第三方库，暂时使用 MD5。如果是用于 FOFA/Shodan 匹配，通常需要使用 mmh3(base64(img_bytes))。
func GetIconHash(imageURL string) (string, error) {
	if imageURL == "" {
		return "", nil
	}

	resp, err := http.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image %s: %w", imageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status code %d when downloading image %s", resp.StatusCode, imageURL)
	}

	hash := md5.New()
	if _, err := io.Copy(hash, resp.Body); err != nil {
		return "", fmt.Errorf("failed to hash image content: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
