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

	return fmt.Sprintf(`你是专业的证件照质量审核员。请严格判断输入图片是否为合格的证件照。

证件照规格要求：%s

重要说明：证件照不要求全身入镜，但必须完整包含整张脸、完整头部以及必要的双肩区域。

必须严格按以下顺序执行：
第一步：硬性准入检查。
第二步：只有硬性准入全部通过后，才允许进行质量评分。

【一、硬性准入检查】
逐项判断并返回对应布尔字段：

1. person_count：画面中真实人物数量，必须恰好为 1。
2. real_person：必须是真人现场照片。空白图、纯背景图、风景图、物品图、卡通图、宠物图、翻拍屏幕或截图均为 false。
3. face_detected：必须能清晰识别到 1 张人脸。
4. face_complete：整张脸必须完整入镜。双眼、鼻子、嘴巴、下巴等主要面部区域不得被画面边缘裁掉，不得因严重遮挡而缺失。只要下半脸、侧脸区域或其他主要面部区域被裁掉，就必须为 false，即使仍能识别人脸也一样。
5. head_complete：完整头部必须在画面内，头顶和头部轮廓不得超出画面边缘。
6. shoulders_visible：左右肩部必须至少部分可见，不能只有脸部局部特写。
7. face_size_appropriate：人脸大小必须适合证件照。局部脸部特写、人脸过大导致脸或头被裁切、人脸过小无法判断时均为 false。
8. face_centered：人脸应基本位于画面中央。轻微偏移可为 true，明显偏移必须为 false。

admission_passed 当且仅当以下条件全部满足时为 true：
- person_count == 1
- real_person == true
- face_detected == true
- face_complete == true
- head_complete == true
- shoulders_visible == true
- face_size_appropriate == true

face_centered 不单独作为硬性准入条件，但必须影响 composition 评分和 issues。

如果 admission_passed=false：
- passed 必须为 false。
- score 必须为 0。
- breakdown 六项必须全部为 0。
- 不得继续评价光线、背景、清晰度、表情等质量问题。
- hard_failures 必须列出最主要的硬性问题，最多 2 条。
- issues 必须优先、逐项复制 hard_failures，禁止用“光线过强”等质量问题替代硬性问题。
- suggestions 必须针对 hard_failures 给出对应的重拍建议。

硬性问题优先级从高到低：
1. 未检测到人物或画面中存在多人
2. 非真人证件照
3. 未检测到清晰人脸
4. 人脸未完整入镜
5. 头部未完整入镜
6. 肩部未完整入镜
7. 人脸大小不合适

【二、质量评分】
只有 admission_passed=true 时才评分，每项 0-100 分：
1. clarity：人脸清晰度
2. lighting：光线是否均匀、是否欠曝或过曝
3. angle：人脸是否正对镜头、是否明显歪斜或侧转
4. background：背景是否干净并符合指定背景色
5. expression：表情是否自然规范
6. composition：人脸是否居中、头顶留白和肩部构图是否合理

总分 score 取六项整数平均值，四舍五入。

admission_passed=true 后，当且仅当同时满足以下条件时 passed=true：
- score >= 70
- 六个维度均 >= 60

【三、问题生成与排序】
1. hard_failures、issues、suggestions 均最多 2 条，每条简短明确。
2. 硬性准入失败时，issues 第一项必须是最高优先级硬性问题。
3. 准入通过后，issues 才可以包含构图、光线、清晰度、背景、角度或表情问题。
4. face_centered=false 时，“人脸未居中”必须优先于光线类问题。
5. 禁止只提示次要问题而遗漏导致不通过的主要问题。
6. 图片合格时，hard_failures、issues、suggestions 返回空数组。

【四、输出格式】
只输出一份严格 JSON，不要输出 Markdown、代码块、解释或任何额外文字；不要缺少或增加字段。

固定字段格式：
{"passed":false,"admission_passed":false,"person_count":1,"real_person":true,"face_detected":true,"face_complete":false,"head_complete":true,"shoulders_visible":false,"face_centered":false,"face_size_appropriate":false,"score":0,"breakdown":{"clarity":0,"lighting":0,"angle":0,"background":0,"expression":0,"composition":0},"hard_failures":["人脸未完整入镜","肩部未完整入镜"],"issues":["人脸未完整入镜","肩部未完整入镜"],"suggestions":["拉远并拍摄完整面部","拉远并露出双肩"]}

再次强调：能够识别人脸不代表人脸完整。只露出额头、眼睛或部分面部的局部特写必须判定 admission_passed=false，并优先提示“人脸未完整入镜”，不得只提示光线问题。

无法判断任何硬性字段时，从严处理，将该字段设为 false。%s`, trimmedSpec, retryHint)
}
