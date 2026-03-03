import { describe, it, expect } from 'vitest'
import zhCN from './lang/zh-CN.js'
import enUS from './lang/en-US.js'

describe('i18n Translation Tests', () => {
  describe('Translation Structure', () => {
    it('should have same top-level keys in both languages', () => {
      const zhKeys = Object.keys(zhCN).sort()
      const enKeys = Object.keys(enUS).sort()

      expect(zhKeys).toEqual(enKeys)
    })

    it('should have same nested keys in common section', () => {
      const zhCommonKeys = Object.keys(zhCN.common).sort()
      const enCommonKeys = Object.keys(enUS.common).sort()

      expect(zhCommonKeys).toEqual(enCommonKeys)
    })

    it('should have same keys in nav section', () => {
      const zhNavKeys = Object.keys(zhCN.nav).sort()
      const enNavKeys = Object.keys(enUS.nav).sort()

      expect(zhNavKeys).toEqual(enNavKeys)
    })

    it('should have same keys in dashboard section', () => {
      const zhKeys = Object.keys(zhCN.dashboard).sort()
      const enKeys = Object.keys(enUS.dashboard).sort()

      expect(zhKeys).toEqual(enKeys)
    })

    it('should have same keys in profile section', () => {
      const zhKeys = Object.keys(zhCN.profile).sort()
      const enKeys = Object.keys(enUS.profile).sort()

      expect(zhKeys).toEqual(enKeys)
    })

    it('should have same keys in provider section', () => {
      const zhKeys = Object.keys(zhCN.provider).sort()
      const enKeys = Object.keys(enUS.provider).sort()

      expect(zhKeys).toEqual(enKeys)
    })

    it('should have health status translation keys', () => {
      const requiredHealthKeys = [
        'healthHealthy',
        'healthUnhealthy', 
        'healthUnknown'
      ]
      
      for (const key of requiredHealthKeys) {
        expect(zhCN.provider[key], `Missing zh provider.${key}`).toBeDefined()
        expect(enUS.provider[key], `Missing en provider.${key}`).toBeDefined()
      }
    })

    it('should have same keys in model section', () => {
      const zhKeys = Object.keys(zhCN.model).sort()
      const enKeys = Object.keys(enUS.model).sort()

      expect(zhKeys).toEqual(enKeys)
    })

    it('should have same keys in route section', () => {
      const zhKeys = Object.keys(zhCN.route).sort()
      const enKeys = Object.keys(enUS.route).sort()

      expect(zhKeys).toEqual(enKeys)
    })

    it('should have same keys in stats section', () => {
      const zhKeys = Object.keys(zhCN.stats).sort()
      const enKeys = Object.keys(enUS.stats).sort()

      expect(zhKeys).toEqual(enKeys)
    })

    it('should have same keys in logs section', () => {
      const zhKeys = Object.keys(zhCN.logs).sort()
      const enKeys = Object.keys(enUS.logs).sort()

      expect(zhKeys).toEqual(enKeys)
    })

    it('should have same keys in settings section', () => {
      const zhKeys = Object.keys(zhCN.settings).sort()
      const enKeys = Object.keys(enUS.settings).sort()

      expect(zhKeys).toEqual(enKeys)
    })

    it('should have same keys in message section', () => {
      const zhKeys = Object.keys(zhCN.message).sort()
      const enKeys = Object.keys(enUS.message).sort()

      expect(zhKeys).toEqual(enKeys)
    })
  })

  describe('Translation Content', () => {
    it('should not have empty strings in Chinese translations', () => {
      const checkEmptyStrings = (obj, path = '') => {
        for (const [key, value] of Object.entries(obj)) {
          const currentPath = path ? `${path}.${key}` : key
          if (typeof value === 'string') {
            expect(value.trim(), `Empty translation at ${currentPath}`).not.toBe('')
          } else if (typeof value === 'object' && value !== null) {
            checkEmptyStrings(value, currentPath)
          }
        }
      }
      checkEmptyStrings(zhCN)
    })

    it('should not have empty strings in English translations', () => {
      const checkEmptyStrings = (obj, path = '') => {
        for (const [key, value] of Object.entries(obj)) {
          const currentPath = path ? `${path}.${key}` : key
          if (typeof value === 'string') {
            expect(value.trim(), `Empty translation at ${currentPath}`).not.toBe('')
          } else if (typeof value === 'object' && value !== null) {
            checkEmptyStrings(value, currentPath)
          }
        }
      }
      checkEmptyStrings(enUS)
    })

    it('should have Chinese characters in Chinese translations', () => {
      const hasChineseChar = (str) => /[\u4e00-\u9fa5]/.test(str)
      
      let chineseCount = 0
      let totalCount = 0

      const countChinese = (obj) => {
        for (const value of Object.values(obj)) {
          if (typeof value === 'string') {
            totalCount++
            if (hasChineseChar(value)) {
              chineseCount++
            }
          } else if (typeof value === 'object' && value !== null) {
            countChinese(value)
          }
        }
      }

      countChinese(zhCN)
      
      // At least 70% of Chinese translations should contain Chinese characters
      expect(chineseCount / totalCount).toBeGreaterThan(0.7)
    })

    it('should have English characters in English translations', () => {
      const hasEnglishChar = (str) => /[a-zA-Z]/.test(str)
      
      let englishCount = 0
      let totalCount = 0

      const countEnglish = (obj) => {
        for (const value of Object.values(obj)) {
          if (typeof value === 'string') {
            totalCount++
            if (hasEnglishChar(value)) {
              englishCount++
            }
          } else if (typeof value === 'object' && value !== null) {
            countEnglish(value)
          }
        }
      }

      countEnglish(enUS)
      
      // At least 90% of English translations should contain English characters
      expect(englishCount / totalCount).toBeGreaterThan(0.9)
    })
  })

  describe('Translation Placeholders', () => {
    it('should have consistent placeholders in both languages', () => {
      const extractPlaceholders = (str) => {
        const matches = str.match(/\{[^}]+\}/g)
        return matches ? matches.sort() : []
      }

      const checkPlaceholders = (zhObj, enObj, path = '') => {
        for (const key of Object.keys(zhObj)) {
          const currentPath = path ? `${path}.${key}` : key
          const zhValue = zhObj[key]
          const enValue = enObj[key]

          if (typeof zhValue === 'string' && typeof enValue === 'string') {
            const zhPlaceholders = extractPlaceholders(zhValue)
            const enPlaceholders = extractPlaceholders(enValue)

            expect(zhPlaceholders, `Placeholder mismatch at ${currentPath}`)
              .toEqual(enPlaceholders)
          } else if (typeof zhValue === 'object' && zhValue !== null) {
            checkPlaceholders(zhValue, enValue, currentPath)
          }
        }
      }

      checkPlaceholders(zhCN, enUS)
    })
  })

  describe('Specific Translation Keys', () => {
    it('should have all required navigation keys', () => {
      const requiredNavKeys = [
        'dashboard',
        'profiles',
        'providers',
        'models',
        'routes',
        'stats',
        'logs',
        'settings'
      ]

      for (const key of requiredNavKeys) {
        expect(zhCN.nav[key], `Missing zh nav.${key}`).toBeDefined()
        expect(enUS.nav[key], `Missing en nav.${key}`).toBeDefined()
      }
    })

    it('should have all required common action keys', () => {
      const requiredCommonKeys = [
        'add',
        'edit',
        'delete',
        'save',
        'cancel',
        'confirm',
        'search',
        'refresh',
        'test',
        'copy'
      ]

      for (const key of requiredCommonKeys) {
        expect(zhCN.common[key], `Missing zh common.${key}`).toBeDefined()
        expect(enUS.common[key], `Missing en common.${key}`).toBeDefined()
      }
    })

    it('should have all required message keys', () => {
      const requiredMessageKeys = [
        'confirmDelete',
        'saveSuccess',
        'saveFailed',
        'deleteSuccess',
        'deleteFailed',
        'copySuccess',
        'copyFailed'
      ]

      for (const key of requiredMessageKeys) {
        expect(zhCN.message[key], `Missing zh message.${key}`).toBeDefined()
        expect(enUS.message[key], `Missing en message.${key}`).toBeDefined()
      }
    })
  })

  describe('Translation Length Check', () => {
    it('should not have excessively long translations', () => {
      const MAX_LENGTH = 200

      const checkLength = (obj, path = '') => {
        for (const [key, value] of Object.entries(obj)) {
          const currentPath = path ? `${path}.${key}` : key
          if (typeof value === 'string') {
            expect(value.length, `Translation too long at ${currentPath}`)
              .toBeLessThanOrEqual(MAX_LENGTH)
          } else if (typeof value === 'object' && value !== null) {
            checkLength(value, currentPath)
          }
        }
      }

      checkLength(zhCN)
      checkLength(enUS)
    })
  })
})
