package pkg

import (
	"fmt"
	"strings"
)

func buildPhotoEvaluationPrompt(photoSpec, retryHint string) string {
	trimmedSpec := strings.TrimSpace(photoSpec)
	if trimmedSpec == "" {
		trimmedSpec = "未提供额外规格，按标准证件照要求判断"
	}

	return fmt.Sprintf(`你是专业的证件照质量审核员。请根据画面中实际可见的内容判断，不要因为审核要求严格而猜测不存在的裁切或遮挡。

证件照规格要求：%s

重要说明：证件照不要求全身入镜，也不要求完整拍到整条肩膀或手臂。只要整张脸、完整头部都在画面内，且左右肩部各有可辨识的部分，即满足对应的构图准入要求。

必须完成两个相互独立的任务：
第一步：只判断人物、脸、头部和肩部等可观察事实。
第二步：无论第一步结果如何，都必须独立完成六项质量评分；不得自行决定准入或通过。

【一、硬性准入检查】
逐项判断并返回对应布尔字段：

1. person_count：画面中真实人物数量，必须恰好为 1。
2. real_person：必须是真人现场照片。空白图、纯背景图、风景图、物品图、卡通图、宠物图、翻拍屏幕或截图均为 false。
3. face_detected：必须能清晰识别到 1 张人脸。
4. face_complete：只判断主要面部区域是否被裁切或严重遮挡。双眼、鼻子、嘴巴和下巴均可见且未超出画面边缘时必须为 true；眼镜、头发或耳朵未完全露出不能据此判为 false。
5. head_complete：完整头部必须在画面内，头顶和头部轮廓不得超出画面边缘。
6. shoulders_visible：左右肩部各有一部分清晰可见时必须为 true，不要求肩膀、手臂或上半身完整入镜；只有缺少一侧肩部或完全看不到肩部时才为 false。
7. face_size_appropriate：人脸大小必须适合证件照。完整头部、整张脸和左右肩部均可见且面部细节足以判断时应为 true；只有局部脸部特写、裁切到脸或头、或人脸小到无法判断时才为 false。
8. face_centered：人脸应基本位于画面中央。轻微偏移可为 true，明显偏移必须为 false。

【判定校准】
- 双眼、鼻子、嘴巴和下巴都在画面内：face_complete=true。
- 头顶以及左右头部轮廓都在画面内：head_complete=true。
- 左右两侧均能看到肩线或肩部的一部分：shoulders_visible=true。
- 完整头部、整张脸和左右肩部均可见，且面部细节清晰可辨：face_size_appropriate=true。
- 背景杂乱、背景色不符、光线问题、服装问题、表情问题、人脸轻微偏移或头顶留白不足只能影响质量评分，不能把上述硬性字段改为 false。
- 可见内容满足以上任一 true 条件时，禁止将该字段设为 false。

这些布尔字段只描述画面事实，不代表最终是否合格。
后台会根据这些字段确定性计算硬性准入结果、硬失败文案、总分和是否通过；你不得返回或猜测这些结果。
face_centered 只影响 composition 评分和质量问题。
背景色不符、背景杂乱、眼镜或镜片反光、光线、服装、表情、角度、居中、构图和文件大小不得改变前述硬性字段。
不要返回 admission_passed、passed、score 或 hard_failures。

【二、质量评分】
无论硬性字段取值如何，都必须独立给出以下每项 0-100 分：
1. clarity：人脸清晰度
2. lighting：光线是否均匀、是否欠曝或过曝
3. angle：人脸是否正对镜头、是否明显歪斜或侧转
4. background：背景是否干净并符合指定背景色
5. expression：表情是否自然规范
6. composition：人脸是否居中、头顶留白和肩部构图是否合理


【三、问题生成与排序】
1. issues 和 suggestions 均最多 2 条，每条简短明确。
2. 这里只填写清晰度、光线、角度、背景、表情或构图等质量问题，不要填写硬性准入问题。
3. face_centered=false 时，“人脸未居中”优先于光线类问题。
4. 没有明显质量问题时，issues 和 suggestions 返回空数组。

【四、输出格式】
只输出一份严格 JSON，不要输出 Markdown、代码块、解释或任何额外文字；不要缺少或增加字段。

- real_person、face_detected、face_complete、head_complete、shoulders_visible、face_centered、face_size_appropriate：布尔值。
- person_count：整数。
- breakdown：必须包含 clarity、lighting、angle、background、expression、composition 六个整数键。
- issues、suggestions：字符串数组。

再次强调：必须先查看画面边缘是否真的裁掉了面部或头部，再决定 face_complete 或 head_complete。能够看到左右肩部的一部分就应将 shoulders_visible 设为 true。

只有相关区域确实不可见、被裁掉、被严重遮挡或模糊到无法辨认时，才将对应硬性字段设为 false；不得仅因不确定而默认 false。%s`, trimmedSpec, retryHint)
}
