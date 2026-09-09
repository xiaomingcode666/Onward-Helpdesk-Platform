import type { ExtractedMessages } from "./types"

export const exportsMessages = {
  "zh-CN": {
    exportsExtract: {
      csvFaultStats: {
        exportCsv: "导出 CSV",
        filename: "产品故障统计-{productId}-{range}.csv",
        header: {
          title: "产品故障统计",
          productId: "产品 ID",
          range: "统计范围",
          generatedAt: "生成时间",
          exportedAt: "导出时间",
          summary: "汇总",
          faultTotal: "故障总数",
          affectedDevices: "影响设备",
        },
        columns: {
          part: "故障部位",
          faultType: "故障类型",
          model: "型号",
          count: "次数",
          share: "占比",
          trend: "趋势",
          severity: "严重度",
        },
      },
      csvResourceTable: {
        exportCsv: "导出 CSV",
        exportedAt: "导出时间",
        recordCount: "记录数",
      },
      csvQuality: {
        filename: "产品质量信号-{productId}.csv",
        title: "产品质量信号",
      },
    },
  },
  "en-US": {
    exportsExtract: {
      csvFaultStats: {
        exportCsv: "Export CSV",
        filename: "Fault-stats-{productId}-{range}.csv",
        header: {
          title: "Product Fault Statistics",
          productId: "Product ID",
          range: "Statistics range",
          generatedAt: "Generated at",
          exportedAt: "Exported at",
          summary: "Summary",
          faultTotal: "Total faults",
          affectedDevices: "Affected devices",
        },
        columns: {
          part: "Part",
          faultType: "Fault type",
          model: "Model",
          count: "Count",
          share: "Share",
          trend: "Trend",
          severity: "Severity",
        },
      },
      csvResourceTable: {
        exportCsv: "Export CSV",
        exportedAt: "Exported at",
        recordCount: "Records",
      },
      csvQuality: {
        filename: "Quality-signals-{productId}.csv",
        title: "Product quality signals",
      },
    },
  },
  "es-ES": {
    exportsExtract: {
      csvFaultStats: {
        exportCsv: "Exportar CSV",
        filename: "Estadisticas-de-fallas-{productId}-{range}.csv",
        header: {
          title: "Estadisticas de fallas del producto",
          productId: "ID del producto",
          range: "Rango de estadisticas",
          generatedAt: "Generado el",
          exportedAt: "Exportado el",
          summary: "Resumen",
          faultTotal: "Total de fallas",
          affectedDevices: "Dispositivos afectados",
        },
        columns: {
          part: "Parte",
          faultType: "Tipo de falla",
          model: "Modelo",
          count: "Cantidad",
          share: "Proporcion",
          trend: "Tendencia",
          severity: "Severidad",
        },
      },
      csvResourceTable: {
        exportCsv: "Exportar CSV",
        exportedAt: "Exportado el",
        recordCount: "Registros",
      },
      csvQuality: {
        filename: "Senales-de-calidad-{productId}.csv",
        title: "Senales de calidad del producto",
      },
    },
  },
} satisfies ExtractedMessages
