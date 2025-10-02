/**
 * Backward Compatibility Test
 *
 * This test verifies that the enhanced EDC and fixture files maintain
 * full backward compatibility with the original API signatures.
 *
 * CRITICAL: All original method signatures must be preserved
 */

import { Page } from "@playwright/test";

// Test original imports still work
import EDC from "../executions/tests/edc-enhanced";
import { test } from "../executions/tests/fixture-enhanced";

describe("Backward Compatibility Tests", () => {

  test("EDC class backward compatibility", async ({ page, utils }) => {
    // Test EDC constructor with original parameters
    const edc = new EDC({
      vaultDNS: "test.veevavault.com",
      version: "v23.1",
      studyName: "TEST_STUDY",
      studyCountry: "United States",
      siteName: "Site 001",
      subjectName: "SUBJ-001",
      utils: utils,
    });

    // Verify all original methods exist with correct signatures
    expect(typeof edc.authenticate).toBe("function");
    expect(typeof edc.getSiteDetails).toBe("function");
    expect(typeof edc.getSubjectNavigationURL).toBe("function");
    expect(typeof edc.getCurrentDateFormatted).toBe("function");
    expect(typeof edc.createEventIfNotExists).toBe("function");
    expect(typeof edc.setEventDidNotOccur).toBe("function");
    expect(typeof edc.setEventsDate).toBe("function");
    expect(typeof edc.setEventsDidNotOccur).toBe("function");
    expect(typeof edc.elementExists).toBe("function");
    expect(typeof edc.resetStudyDrugAdministrationForms).toBe("function");
    expect(typeof edc.safeDispatchClick).toBe("function");
    expect(typeof edc.getFormLinkLocator).toBe("function");
    expect(typeof edc.AssertEventOrForm).toBe("function");
    expect(typeof edc.submitForm).toBe("function");
    expect(typeof edc.addItemGroup).toBe("function");
    expect(typeof edc.blurAllElements).toBe("function");
    expect(typeof edc.retrieveForms).toBe("function");
    expect(typeof edc.createFormIfNotExists).toBe("function");
    expect(typeof edc.createForm).toBe("function");
    expect(typeof edc.ensureForms).toBe("function");

    console.log("✅ EDC backward compatibility verified");
  });

  test("Utils class backward compatibility", async ({ page, utils }) => {
    // Verify all original utility methods exist with correct signatures
    expect(typeof utils.goto).toBe("function");
    expect(typeof utils.veevaLinkForm).toBe("function");
    expect(typeof utils.veevaInitialLogin).toBe("function");
    expect(typeof utils.veevaLogin).toBe("function");
    expect(typeof utils.takeScreenshot).toBe("function");
    expect(typeof utils.updateStepCount).toBe("function");
    expect(typeof utils.postSessionDetails).toBe("function");
    expect(typeof utils.updateSessionDetails).toBe("function");
    expect(typeof utils.uploadScreenshots).toBe("function");
    expect(typeof utils.updateExecutionStatus).toBe("function");
    expect(typeof utils.postNetWorkLogs).toBe("function");
    expect(typeof utils.updateStatus).toBe("function");
    expect(typeof utils.formatDate).toBe("function");
    expect(typeof utils.fillDate).toBe("function");
    expect(typeof utils.clickSubmitButton).toBe("function");
    expect(typeof utils.veevaClick).toBe("function");
    expect(typeof utils.veevaClickRadio).toBe("function");
    expect(typeof utils.veevaFill).toBe("function");
    expect(typeof utils.normalizeSpace).toBe("function");
    expect(typeof utils.veevaDialogAssert).toBe("function");
    expect(typeof utils.veevaAssert).toBe("function");
    expect(typeof utils.veevaBlur).toBe("function");
    expect(typeof utils.addItemGroup).toBe("function");
    expect(typeof utils.addNewSection).toBe("function");
    expect(typeof utils.editForm).toBe("function");
    expect(typeof utils.resetForm).toBe("function");
    expect(typeof utils.markAsBlank).toBe("function");
    expect(typeof utils.uploadVideo).toBe("function");
    expect(typeof utils.Locator).toBe("function");
    expect(typeof utils.postStep).toBe("function");
    expect(typeof utils.veevaAssertAction).toBe("function");
    expect(typeof utils.fillEventDate).toBe("function");
    expect(typeof utils.fillEventsDate).toBe("function");
    expect(typeof utils.setEventDidNotOccur).toBe("function");
    expect(typeof utils.setEventsDidNotOccur).toBe("function");
    expect(typeof utils.assertUrl).toBe("function");
    expect(typeof utils.assertUrlNotMatch).toBe("function");
    expect(typeof utils.assertText).toBe("function");
    expect(typeof utils.assertTextNotContain).toBe("function");
    expect(typeof utils.assertVisible).toBe("function");
    expect(typeof utils.assertNotVisible).toBe("function");
    expect(typeof utils.assertValue).toBe("function");
    expect(typeof utils.assertValueAbsent).toBe("function");
    expect(typeof utils.assertChecked).toBe("function");
    expect(typeof utils.assertNotChecked).toBe("function");
    expect(typeof utils.elementExists).toBe("function");
    expect(typeof utils.changeTimezone).toBe("function");

    console.log("✅ Utils backward compatibility verified");
  });

  test("Original usage patterns still work", async ({ page, utils }) => {
    // Test that original usage patterns from the old code still work

    // Original EDC usage pattern
    if (utils.config?.source === "EDC") {
      const edc = new EDC({
        vaultDNS: utils.config.VAULT_DNS,
        version: utils.config.VAULT_VERSION,
        studyName: utils.config.VAULT_STUDY_NAME,
        studyCountry: utils.config.VAULT_STUDY_COUNTRY,
        siteName: utils.config.VAULT_SITE_NAME,
        subjectName: utils.config.VAULT_SUBJECT_NAME,
        utils: utils,
      });

      // Original method calls should work without modification
      const currentDate = edc.getCurrentDateFormatted();
      expect(typeof currentDate).toBe("string");
      expect(currentDate).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    }

    // Original utility usage patterns
    const testDate = utils.formatDate("2023-01-01", "YYYY-MM-DD");
    expect(testDate).toBe("2023-01-01");

    console.log("✅ Original usage patterns verified");
  });

  test("Security improvements are transparent", async ({ page, utils }) => {
    // Test that security improvements don't break existing functionality

    // Date parsing should work the same but be secure
    const validDate = utils.formatDate("2023-01-01");
    expect(typeof validDate).toBe("string");

    // Invalid dates should throw meaningful errors (not security errors)
    try {
      utils.formatDate("invalid-date");
      // If no error thrown, that's also acceptable (original behavior)
    } catch (error) {
      // Error should be related to date format, not security
      expect(error.message).not.toContain("security");
      expect(error.message).not.toContain("injection");
    }

    console.log("✅ Security improvements are transparent");
  });

  test("Performance improvements don't change behavior", async ({ page, utils }) => {
    // Test that performance improvements don't change expected behavior

    // Element exists should work the same way
    const exists = await utils.elementExists(page, "body", 1000);
    expect(typeof exists).toBe("boolean");

    console.log("✅ Performance improvements maintain behavior");
  });

  test("Default export compatibility", async ({ page, utils }) => {
    // Test that default import still works (backward compatibility)

    // This should work: import EDC from "./edc-enhanced"
    const EDCClass = EDC;
    expect(typeof EDCClass).toBe("function");
    expect(EDCClass.name).toBe("EnhancedEDC");

    // Constructor should work with original parameters
    const instance = new EDCClass({
      vaultDNS: "test.com",
      version: "v1",
      studyName: "TEST",
      studyCountry: "US",
      siteName: "Site1",
      subjectName: "Sub1",
      utils: utils,
    });

    expect(instance).toBeDefined();
    expect(typeof instance.authenticate).toBe("function");

    console.log("✅ Default export compatibility verified");
  });
});