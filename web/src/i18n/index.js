import { createI18n } from 'vue-i18n'
import zhCN from './lang/zh-CN.js'
import enUS from './lang/en-US.js'

const messages = {
  'zh-CN': zhCN,
  'en-US': enUS,
  'zh': zhCN,
  'en': enUS,
}

const i18n = createI18n({
  legacy: false,
  locale: localStorage.getItem('locale') || navigator.language || 'zh-CN',
  fallbackLocale: 'en-US',
  messages,
})

export default i18n
