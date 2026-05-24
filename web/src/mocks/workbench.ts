import type { AnnotationHistory, FormSchema, TaskBundle } from '@/types'

export const mockDatasetName = '高考专业库'

// 所有任务共享的 form_schema（v3）
const formSchema: FormSchema = {
  version: 3,
  source_fields: [
    { code: 'id', type: 'int', widget: 'Input', label: 'ID' },
    { code: 'title', type: 'text', widget: 'Input', label: '专业名称', primary: true },
    { code: 'body', type: 'text', widget: 'TextArea', label: '专业介绍', primary: true },
    { code: 'category', type: 'text', widget: 'Input', label: '原学科门类' },
  ],
  annotation_fields: [
    {
      code: 'discipline',
      label: '学科归类',
      widget: 'Select',
      required: true,
      group: 'core',
      options: [
        { value: 'theory', label: '理论型' },
        { value: 'engineering', label: '工程型' },
        { value: 'interdisciplinary', label: '交叉型' },
        { value: 'applied', label: '应用型' },
      ],
      hotkeys: { Q: 'theory', W: 'engineering', E: 'interdisciplinary', R: 'applied' },
    },
    {
      code: 'difficulty',
      label: '学习难度',
      widget: 'Rating',
      required: true,
      min: 1,
      max: 5,
      group: 'core',
    },
    {
      code: 'confidence',
      label: '标注置信度',
      widget: 'Confidence',
      min: 0,
      max: 1,
      step: 0.1,
      default: 0.7,
      group: 'core',
    },
    {
      code: 'note',
      label: '备注',
      widget: 'TextArea',
      max_length: 500,
      group: 'extra',
    },
  ],
}

export const mockQueue: TaskBundle[] = [
  {
    task: {
      id: 88123,
      dataset_id: 1,
      source_row_pk: '4501',
      status: 'CLAIMED',
      round: 2,
      assigned_to: 7,
      lease_expires_at: null, // 运行时由 LeaseTimer 设定
    },
    source_row: {
      id: 4501,
      title: '计算机科学与技术',
      body: '本专业培养具备扎实的数学与计算机科学理论基础，系统掌握计算机硬件、软件与网络技术，能够在科研院所、企事业单位及行政部门从事计算机教学、科学研究与应用开发的高级专门人才。主干课程包括数据结构、操作系统、计算机组成原理、编译原理、计算机网络、数据库系统与人工智能导论等。毕业生既可进入互联网与软件行业从事系统开发，也可继续深造攻读计算机及相关交叉学科的硕士、博士学位。',
      category: '工学',
    },
    form_schema: formSchema,
    ai_suggestion: {
      discipline: 'engineering',
      difficulty: 4,
      confidence: 0.82,
      _source: 'ai',
      _ai_confidence: 0.82,
      _ai_reasoning: '正文反复强调“应用开发”“系统开发”“硬件软件网络技术”，工程实践导向明显，倾向工程型。',
    },
  },
  {
    task: {
      id: 88124,
      dataset_id: 1,
      source_row_pk: '4502',
      status: 'CLAIMED',
      round: 1,
      assigned_to: 7,
      lease_expires_at: null,
    },
    source_row: {
      id: 4502,
      title: '哲学',
      body: '哲学专业旨在培养具有较高理论素养与思辨能力、系统掌握中外哲学史与哲学基本原理的研究型人才。课程涵盖马克思主义哲学、中国哲学史、西方哲学史、伦理学、逻辑学、美学与科学技术哲学等。本专业注重经典文本的精读与批判性思维训练，毕业生多在高校、研究机构、新闻出版与行政管理等领域从事教学、研究与文化工作。',
      category: '哲学',
    },
    form_schema: formSchema,
    ai_suggestion: {
      discipline: 'theory',
      difficulty: 3,
      confidence: 0.78,
      _source: 'ai',
      _ai_confidence: 0.78,
      _ai_reasoning: '强调“理论素养”“思辨能力”“经典文本精读”，无明显应用/工程指向，倾向理论型。',
    },
  },
  {
    task: {
      id: 88125,
      dataset_id: 1,
      source_row_pk: '4503',
      status: 'CLAIMED',
      round: 1,
      assigned_to: 7,
      lease_expires_at: null,
    },
    source_row: {
      id: 4503,
      title: '生物医学工程',
      body: '生物医学工程是综合生命科学、医学与工程技术的交叉学科，培养既懂医学又掌握工程方法、能够从事医疗仪器研发、医学影像处理、生物信息分析与康复工程的复合型人才。主干课程包括电子技术、信号与系统、医学影像、生物力学、传感器原理与医学仪器设计。毕业生广泛就业于医疗器械企业、医院设备科与生物技术公司。',
      category: '工学',
    },
    form_schema: formSchema,
    ai_suggestion: {
      discipline: 'interdisciplinary',
      difficulty: 5,
      confidence: 0.9,
      _source: 'ai',
      _ai_confidence: 0.9,
      _ai_reasoning: '正文明确为“综合生命科学、医学与工程技术的交叉学科”，交叉型证据充分。',
    },
  },
  {
    task: {
      id: 88126,
      dataset_id: 1,
      source_row_pk: '4504',
      status: 'CLAIMED',
      round: 1,
      assigned_to: 7,
      lease_expires_at: null,
    },
    source_row: {
      id: 4504,
      title: '会计学',
      body: '会计学专业培养具备管理、经济、法律和会计学知识与能力，能在企事业单位、会计师事务所及政府部门从事会计实务、审计与财务管理工作的应用型专门人才。课程包括财务会计、成本会计、管理会计、审计学、财务管理、税法与经济法。专业强调实务操作与职业资格衔接，毕业生可考取注册会计师等职业资格。',
      category: '管理学',
    },
    form_schema: formSchema,
    ai_suggestion: {
      discipline: 'applied',
      difficulty: 3,
      confidence: 0.85,
      _source: 'ai',
      _ai_confidence: 0.85,
      _ai_reasoning: '强调“实务操作”“职业资格衔接”“应用型专门人才”，应用型导向明确。',
    },
  },
]

// 左栏历史回显（按 task.id）
export const mockHistory: Record<number, AnnotationHistory[]> = {
  88123: [{ round: 1, annotator: '李明', created_at: '2 天前', superseded: true }],
}
