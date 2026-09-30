/**
 * 厨房专区（KitchenPage）：家庭报饭汇总 + 灵感搭配 + 私房菜谱库 + 免脏屏大字模式
 */
import { useCallback, useEffect, useState } from 'react'

import { api, unwrap } from '../../api/client'

import styles from './KitchenPage.module.css'

interface Step {
  step: number
  title: string
  detail: string
  tips?: string
}

interface Recipe {
  id: string
  name: string
  category: string
  timeMinutes: number
  ingredients: string[]
  tips: string
  steps: Step[]
}

interface MealMember {
  member_id: string
  name: string
  at_home: boolean
}

interface MealData {
  date: string
  at_home_count: number
  not_at_home_count: number
  members: MealMember[]
}

const DEFAULT_RECIPES: Recipe[] = [
  {
    id: 'r1',
    name: '家常红烧肉',
    category: '拿手荤菜',
    timeMinutes: 50,
    ingredients: ['五花肉 500g', '冰糖 30g', '生抽 2勺', '老抽 1勺', '八角 2个', '生姜 4片'],
    tips: '五花肉切块后一定要厨房纸吸干水分，否则下油锅会剧烈溅油；小火炒糖色见细小黄泡立即下肉。',
    steps: [
      { step: 1, title: '冷水焯肉', detail: '肉块冷水下锅，加姜片料酒大火煮沸3分钟，捞出温水冲净沥干。', tips: '切忌冷水冲热肉，肉质会发柴。' },
      { step: 2, title: '小火炒糖色', detail: '热锅倒少许底油，入冰糖小火慢慢慢熬化，变为琥珀色起密泡。', tips: '不要等变黑，发黑就苦了。' },
      { step: 3, title: '煸炒上色', detail: '下肉块快速翻炒，让每块肉裹匀糖色，煸出部分油脂。' },
      { step: 4, title: '加水慢炖', detail: '倒入足量开水没过肉块，加生抽、老抽、八角，大火烧开转极小火焖炖40分钟。', tips: '必须加开水，不能加凉水！' },
      { step: 5, title: '大火收汁', detail: '开盖挑出八角姜片，大火不断翻炒至汤汁浓稠红亮裹在肉上即可关火。' },
    ],
  },
  {
    id: 'r2',
    name: '清蒸鲈鱼',
    category: '快手海鲜',
    timeMinutes: 15,
    ingredients: ['鲈鱼 1条', '大葱 2根', '生姜 1块', '蒸鱼豉油 3勺', '食用油 2勺'],
    tips: '水开之后再放鱼！大火蒸整8分钟立即关火虚蒸2分钟，蒸出来的原汁一定要倒掉，那是腥味来源。',
    steps: [
      { step: 1, title: '鱼身划刀', detail: '鱼身两面各斜划三刀，塞姜片，抹少许盐和料酒腌制5分钟。' },
      { step: 2, title: '沸水上锅', detail: '蒸锅水大火烧大沸，盘底垫葱段防粘，放入鲈鱼大火蒸整8分钟。' },
      { step: 3, title: '倒掉盘中水', detail: '关火虚蒸2分钟出锅，倒掉盘里的蒸鱼汤水，挑出旧姜葱。' },
      { step: 4, title: '热油激香', detail: '铺上新鲜葱丝红椒丝，淋3勺蒸鱼豉油，烧热油泼在葱丝上激发出香味。' },
    ],
  },
  {
    id: 'r3',
    name: '酸辣土豆丝',
    category: '家常素菜',
    timeMinutes: 10,
    ingredients: ['土豆 2个', '干辣椒 3个', '花椒 10粒', '白醋 2勺', '大蒜 2瓣'],
    tips: '土豆丝必须多淘洗两遍洗净表面淀粉，捞出沥干；出锅前沿锅边再烹入一勺白醋（叫回锅醋）才够脆爽！',
    steps: [
      { step: 1, title: '切丝洗淀粉', detail: '土豆去皮切均匀细丝，清水淘洗两遍直至水清澈，捞出彻底沥干水分。' },
      { step: 2, title: '爆香花椒辣椒', detail: '热锅凉油，下花椒粒炸香捞出不用，再下干辣椒段与蒜片爆香。' },
      { step: 3, title: '大火快炒', detail: '转最大火倒入土豆丝，立刻顺锅边淋入1勺白醋，快速大火翻炒1分钟。' },
      { step: 4, title: '调味出锅', detail: '加半勺盐、少许糖提鲜，关火前再淋半勺白醋翻匀出锅。' },
    ],
  },
]

export function KitchenPage() {
  const [recipes] = useState<Recipe[]>(DEFAULT_RECIPES)
  const [activeDish, setActiveDish] = useState<Recipe | null>(DEFAULT_RECIPES[0])
  const [activeStepIndex, setActiveStepIndex] = useState(1)
  const [isBigFontMode, setIsBigFontMode] = useState(false)
  const [suggestIndex, setSuggestIndex] = useState(0)

  // 真实报饭数据
  const [mealData, setMealData] = useState<MealData | null>(null)
  const [mealLoading, setMealLoading] = useState(true)
  const [mealSubmitting, setMealSubmitting] = useState(false)

  const fetchMeals = useCallback(async () => {
    try {
      const res = await api.GET('/meals')
      const d = unwrap(res)
      setMealData(d)
    } catch {
      setMealData(null)
    } finally {
      setMealLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchMeals()
  }, [fetchMeals])

  const handleReportMeal = async (atHome: boolean) => {
    setMealSubmitting(true)
    try {
      await api.POST('/meals', {
        body: { at_home: atHome },
      })
      await fetchMeals()
    } catch (err) {
      const msg = err instanceof Error ? err.message : '报饭失败'
      alert(msg)
    } finally {
      setMealSubmitting(false)
    }
  }

  const SUGGEST_COMBOS = [
    { main: '红烧肉', side: '酸辣土豆丝', soup: '紫菜蛋花汤' },
    { main: '清蒸鲈鱼', side: '白灼菜心', soup: '西红柿蛋汤' },
    { main: '可乐鸡翅', side: '手撕包菜', soup: '玉米排骨汤' },
  ]

  const currentCombo = SUGGEST_COMBOS[suggestIndex % SUGGEST_COMBOS.length]
  const currentStep = activeDish?.steps[activeStepIndex]

  const handleStartCooking = (recipe: Recipe) => {
    setActiveDish(recipe)
    setActiveStepIndex(0)
    setIsBigFontMode(true)
  }

  const atHomeCount = mealData?.at_home_count ?? 0
  const notAtHomeCount = mealData?.not_at_home_count ?? 0
  const members = mealData?.members ?? []
  const atHomeMembers = members.filter((m) => m.at_home)
  const notAtHomeMembers = members.filter((m) => !m.at_home)

  return (
    <div className={styles.container}>
      <h1 className={styles.title}>厨房与用餐</h1>

      {/* 模块 1：今晚就餐看板（实时接口数据） */}
      <div className={styles.mealCard}>
        <div className={styles.mealHeader}>
          <span className={styles.mealTitle}>🍽️ 今晚用餐申报</span>
          <span className={styles.mealDate}>{mealData?.date || '今日'}</span>
        </div>
        
        <div className={styles.mealSummary}>
          {mealLoading ? (
            <span style={{ color: '#8e8e93', fontSize: 14 }}>加载中…</span>
          ) : (
            <>
              <div className={styles.mealCountText}>
                今晚 <strong style={{ color: '#34c759', fontSize: 20 }}>{atHomeCount}</strong> 人在家吃
                {notAtHomeCount > 0 && <span style={{ color: '#8e8e93', fontSize: 14, marginLeft: 8 }}>· {notAtHomeCount} 人不在家</span>}
              </div>
              <div className={styles.mealMembersRow}>
                {atHomeMembers.map((m) => (
                  <span key={m.member_id} className={styles.memberTagOn}>✓ {m.name} 在家</span>
                ))}
                {notAtHomeMembers.map((m) => (
                  <span key={m.member_id} className={styles.memberTagOff}>✗ {m.name} 外出</span>
                ))}
                {members.length === 0 && (
                  <span style={{ fontSize: 13, color: '#8e8e93' }}>今日暂无申报，点击下方一键报饭</span>
                )}
              </div>
            </>
          )}
        </div>

        <div className={styles.mealActions}>
          <button
            type="button"
            className={`${styles.mealBtn} ${styles.mealBtnOn}`}
            disabled={mealSubmitting}
            onClick={() => handleReportMeal(true)}
          >
            🏠 我在家吃
          </button>
          <button
            type="button"
            className={`${styles.mealBtn} ${styles.mealBtnOff}`}
            disabled={mealSubmitting}
            onClick={() => handleReportMeal(false)}
          >
            🚪 我不在家吃
          </button>
        </div>
      </div>

      {/* 模块 2：正在下厨卡片 */}
      {activeDish && (
        <div className={styles.activeCard}>
          <div className={styles.activeHeader}>
            <span className={styles.activeBadge}>🍳 正在下厨中</span>
            <span style={{ fontSize: 13, opacity: 0.9 }}>
              步骤 {activeStepIndex + 1} / {activeDish.steps.length}
            </span>
          </div>
          <div className={styles.activeDish}>{activeDish.name}</div>
          <div className={styles.activeStep}>
            <strong>第 {activeStepIndex + 1} 步 · {currentStep?.title}：</strong>
            {currentStep?.detail}
          </div>
          <div className={styles.activeActions}>
            <button
              type="button"
              className={styles.modeBtn}
              onClick={() => setIsBigFontMode(true)}
            >
              📱 开启做饭大字免脏屏模式
            </button>
            <button
              type="button"
              className={styles.finishBtn}
              onClick={() => setActiveDish(null)}
            >
              完成
            </button>
          </div>
        </div>
      )}

      {/* 模块 3：今天吃啥灵感搭配 */}
      <div className={styles.section}>今天吃啥 · 灵感搭配</div>
      <div className={styles.suggestCard}>
        <div className={styles.suggestRow}>
          <span className={styles.suggestLabel}>今日三菜一汤推荐</span>
          <button
            type="button"
            className={styles.refreshBtn}
            onClick={() => setSuggestIndex((prev) => prev + 1)}
          >
            🎲 换一组
          </button>
        </div>
        <div className={styles.suggestList}>
          <span className={styles.suggestChip}>🥩 {currentCombo.main}</span>
          <span className={styles.suggestChip}>🥬 {currentCombo.side}</span>
          <span className={styles.suggestChip}>🥣 {currentCombo.soup}</span>
        </div>
      </div>

      {/* 模块 4：家庭私房菜谱库 */}
      <div className={styles.section}>家庭私房菜谱（{recipes.length}）</div>
      <div className={styles.recipeList}>
        {recipes.map((r) => (
          <div key={r.id} className={styles.recipeCard}>
            <div className={styles.recipeTop}>
              <div>
                <span className={styles.recipeName}>{r.name}</span>
                <span style={{ fontSize: 12, color: '#8e8e93', marginLeft: 8 }}>
                  约 {r.timeMinutes} 分钟 · {r.category}
                </span>
              </div>
              <button
                type="button"
                className={styles.cookNowBtn}
                onClick={() => handleStartCooking(r)}
              >
                开始做这道菜
              </button>
            </div>
            <div className={styles.recipeMeta}>
              {r.ingredients.map((ing) => (
                <span key={ing} className={styles.ingredientTag}>
                  {ing}
                </span>
              ))}
            </div>
            {r.tips && (
              <div className={styles.tipsBox}>
                <strong>秘诀：</strong>
                {r.tips}
              </div>
            )}
          </div>
        ))}
      </div>

      {/* 全屏做饭大字免脏屏模式 */}
      {isBigFontMode && activeDish && currentStep && (
        <div className={styles.fullscreenModal}>
          <div className={styles.modalHeader}>
            <div className={styles.modalDish}>{activeDish.name}</div>
            <button
              type="button"
              className={styles.closeBtn}
              onClick={() => setIsBigFontMode(false)}
            >
              ✕
            </button>
          </div>
          <div className={styles.modalBody}>
            <div className={styles.stepNumber}>
              STEP {activeStepIndex + 1} OF {activeDish.steps.length}
            </div>
            <div className={styles.stepContent}>
              {currentStep.title}：{currentStep.detail}
            </div>
            {currentStep.tips && (
              <div className={styles.stepTip}>💡 提示：{currentStep.tips}</div>
            )}
          </div>
          <div className={styles.modalControls}>
            <button
              type="button"
              className={styles.navStepBtn}
              disabled={activeStepIndex === 0}
              onClick={() => setActiveStepIndex((prev) => Math.max(0, prev - 1))}
            >
              上一步
            </button>
            <button
              type="button"
              className={styles.primaryStepBtn}
              onClick={() => {
                if (activeStepIndex < activeDish.steps.length - 1) {
                  setActiveStepIndex((prev) => prev + 1)
                } else {
                  setIsBigFontMode(false)
                  setActiveDish(null)
                }
              }}
            >
              {activeStepIndex < activeDish.steps.length - 1 ? '下一步' : '大功告成'}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
