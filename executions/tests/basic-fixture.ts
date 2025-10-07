import { Page, test as base, expect } from "@playwright/test";
import EDC from "./edc";

import { Agent, fetch, setGlobalDispatcher } from "undici";
setGlobalDispatcher(new Agent({ connect: { timeout: 60_000 } }));
const machineConfig = require("../../configuration/machine_config.json");

type EDCDetails = {
  site: string;
  subject_numbers: string[];
};

let edcDetails: EDCDetails;
try {
  edcDetails = require("../edc.json");
} catch (error) {}
declare global {
  interface process {
    env: {
      TESTLAB: string;
      BASE_PATH: string;
      ELEMENT_TIMEOUT: string;
      PARALLELISM_ENABLED: string;
      TEST_PARALLEL_INDEX: string;
      WORKERS: string;
    };
  }
}

type Config = {
  org_id: string;
  project_id: string;
  duration: number;
  app_id: string;
  testlab: string;
  execution_id: string;
  testcase_id: string;
  testsuite_id: string;
  testplan_id: string;
  is_adhoc: boolean;
  is_prerequisite: boolean;
  parent_testcase_id: string;
  screenshot_type: string;
  step_count: number;
  status: string;
  abort_on_test_failure: boolean;
  abort_on_pre_requisite_failure: boolean;
  source: string;
  VAULT_DNS: string;
  VAULT_VERSION: string;
  VAULT_STUDY_NAME: string;
  VAULT_STUDY_COUNTRY: string;
  VAULT_SITE_NAME: string;
  VAULT_SUBJECT_NAME: string;
  VAULT_USER_NAME: string;
  VAULT_PASSWORD: string;
  EDC_VEEVA_LOGIN_URL: string;
  video_path: string;
};

class Utils {
  public port = machineConfig.listen;
  private basePath = "agent";
  public baseUrl = `http://localhost${this.port}/${this.basePath}/v1`;
  public machineId = "";
  public config!: Config;
  public testlab = "local";
  public isStatusUpdated = false;
  public parallelismEnabled = false;
  public workerIndex = parseInt(process.env.TEST_PARALLEL_INDEX ?? "0");
  public edc!: EDC;
  public timezone: string = "UTC";
  private edcFormDetails!: {
    eventGroupId: string;
    eventId: string;
    formId: string;
    formSequenceIndex: number;
    resetForm: boolean;
  };
  public edcSubjectDetails?: EDCDetails;
  public formsReset: string[] = [];

  public async goto(page: Page, url?: string) {
    if (!url) {
      throw new Error("Cannot navigate as url is empty");
    }
    if (this.config.source !== "EDC") {
      await page.goto(url);
    } else {
      if (!url) {
        console.log("url is empty");
        return;
      }
      const navigation_details = JSON.parse(url);

      console.log("navigation details", navigation_details);
      const {
        eventGroupName,
        eventName,
        formName,
        eventGroupId,
        eventId,
        formId,
        formSequenceIndex = 1,
        resetForm = true,
        isSubjectNumberForm = false,
        isRelatedToStudyTreatment = false,
      } = navigation_details;

      if (this.edc !== undefined) {
        console.log(`Submitting the form ${this.edcFormDetails.formId}`);
        await this.clickSubmitButton(page, "");
        console.log(`form submitted`);

        if (!formName) {
          return;
        }

        const formLinkInfo = await this.edc.getFormLinkLocator({
          page,
          navigation_details,
        });

        if (!formLinkInfo.locatorExists) {
          console.log("failed to navigate to form");
          throw new Error(
            `Form Navigation Failed for ${formName}${
              formLinkInfo.error ? " due to " + formLinkInfo.error : ""
            }`
          );
        }

        await page.waitForLoadState("networkidle");
        await page.waitForLoadState("domcontentloaded");

        await this.veevaBlur(page, "");

        await page.waitForTimeout(2000);
        await this.resetForm(page, resetForm);

        this.edcFormDetails = {
          eventGroupId,
          eventId,
          formId,
          formSequenceIndex,
          resetForm,
        };

        return;
      }

      this.edcFormDetails = {
        eventGroupId,
        eventId,
        formId,
        formSequenceIndex,
        resetForm,
      };
      console.log(
        this.parallelismEnabled,
        this.edcSubjectDetails,
        this.workerIndex
      );
      if (
        this.parallelismEnabled &&
        this.edcSubjectDetails &&
        this.workerIndex < this.edcSubjectDetails.subject_numbers.length
      ) {
        const subjectName =
          this.edcSubjectDetails.subject_numbers[this.workerIndex];
        console.log(`subject name ${subjectName}`);
        await page.waitForTimeout(10000);
        this.edc = new EDC({
          vaultDNS: this.config.VAULT_DNS,
          version: this.config.VAULT_VERSION,
          studyName: this.config.VAULT_STUDY_NAME,
          studyCountry: this.config.VAULT_STUDY_COUNTRY,
          siteName: this.edcSubjectDetails.site,
          subjectName: subjectName,
          utils: this,
        });
      } else {
        this.edc = new EDC({
          vaultDNS: this.config.VAULT_DNS,
          version: this.config.VAULT_VERSION,
          studyName: this.config.VAULT_STUDY_NAME,
          studyCountry: this.config.VAULT_STUDY_COUNTRY,
          siteName: this.config.VAULT_SITE_NAME,
          subjectName: this.config.VAULT_SUBJECT_NAME,
          utils: this,
        });
      }

      const authenticated = await this.edc.authenticate(
        this.config.VAULT_USER_NAME,
        this.config.VAULT_PASSWORD
      );

      if (!authenticated) {
        console.error("failed to authenticate to vault api");
        throw new Error(
          `Unable To Login. Please check EDC Integration Details`
        );
      }

      const siteDetails = await this.edc.getSiteDetails();

      if (!siteDetails) {
        console.log("failed to get site details");
        throw new Error(
          "Unable To Get Site Details. Please check EDC Integration Details."
        );
      }

      this.timezone = this.extractTimezone(siteDetails.timezone);

      const subjectNavigationURL = await this.edc.getSubjectNavigationURL();

      if (!subjectNavigationURL) {
        console.log("failed to get casebook navigation url");
        throw new Error(
          "Unable To Navigate to Casebook. Please check EDC Integration Details."
        );
      }

      await page.goto(subjectNavigationURL);
      await page.waitForLoadState("load");
      await this.veevaLogin(page);
      await page.waitForLoadState("domcontentloaded");

      if (formName) {
        const formLinkInfo = await this.edc.getFormLinkLocator({
          page,
          navigation_details,
        });

        if (!formLinkInfo.locatorExists) {
          console.log("failed to navigate to form");
          throw new Error(`Form Navigation Failed for ${formName}.`);
        }

        await this.veevaBlur(page, "");

        await page.waitForLoadState("domcontentloaded");
        await this.resetForm(page, resetForm, isSubjectNumberForm);
      }
    }
  }

  public async veevaLinkForm(
    page: Page,
    xpath: string,
    formDetailsString: string
  ) {
    if (!xpath) {
      console.log(`empty selector in veeva link form`);
      return;
    }
    try {
      const formDetails = JSON.parse(formDetailsString);

      const {
        eventGroupName,
        eventName,
        formName,
        eventGroupId,
        eventId,
        formId,
        formSequenceIndex = 1,
        resetForm = true,
        isSubjectNumberForm = false,
        isRelatedToStudyTreatment = false,
      } = formDetails;
      await this.edc.createFormIfNotExists({
        eventGroupId,
        eventId,
        formId,
        formSequenceIndex,
      });

      const checkBoxLocator = `//div[contains(@class, 'cdm-linkforms-editor-dialog')]//div[contains(@class, 'cdm-linkforms-grid')]//div[contains(@class, 'vv-data-grid-row')][1]//input[@type= 'checkbox']`;

      const saveButtonLocator = `//footer//button[@type='button'][contains(text(), Save)]`;

      await this.veevaClick(page, xpath);

      await page.waitForTimeout(1000);

      await this.veevaClick(page, checkBoxLocator);

      await page.waitForTimeout(1000);

      await this.veevaClick(page, saveButtonLocator);
    } catch (e) {
      console.log(`error in veeva link form ${e}`);
    }
  }

  public async veevaInitialLogin(page: Page) {
    console.log("login url", this.config.EDC_VEEVA_LOGIN_URL);
    if (!this.config.EDC_VEEVA_LOGIN_URL) {
      throw new Error(
        "Login Url Is Empty. Please Check EDC Integration Details."
      );
    }
    await page.goto(this.config.EDC_VEEVA_LOGIN_URL);
    await page.fill("//*[@id='j_username']", this.config.VAULT_USER_NAME);
    await page
      .locator("//*[contains(text(),'Continue')]")
      .dispatchEvent("click");
    await page.waitForLoadState("domcontentloaded");
    await page.fill("//*[@id='j_password']", this.config.VAULT_PASSWORD);
    await page.locator("//*[contains(text(),'Log In')]").click();
    await page.waitForLoadState("domcontentloaded");
    await page.waitForTimeout(2000);
  }

  public async veevaLogin(page: Page) {
    try {
      await page.waitForURL((url) => url.pathname.includes("/login"), {
        timeout: 5000,
      });
    } catch (e) {
      console.log("Did not navigated to login");
    }
    const url = new URL(page.url());
    console.log("page url", url.pathname);
    if (url.pathname.includes("/login")) {
      console.log("Entering j_password");
      // await page.fill(`//*[@id='j_username']`, this.config.VAULT_USER_NAME);
      // await page
      //   .locator(`//*[contains(text(),'Continue')]`)
      //   .dispatchEvent("click");
      await page.fill(`//*[@id='j_password']`, this.config.VAULT_PASSWORD);
      await page.locator("//*[contains(text(),'Log In')]").click();
      // await this.clickSubmitButton(page, `//*[contains(text(),'Log In')]`);
    }
  }

  public async takeScreenshot(page: Page, config: Config) {
    if (config.screenshot_type == "disabled") {
      return;
    }

    if (config.screenshot_type == "error-only" && config.status != "failed") {
      return;
    }

    const timeStamp = new Date().getTime();
    var screenshotPath = `./assets/screenshots/${config.execution_id}`;
    if (!config.is_adhoc) {
      screenshotPath += `/${config.testplan_id}/${this.machineId}/${config.testsuite_id}/${config.testcase_id}`;
    } else if (config.is_prerequisite) {
      screenshotPath += `/${config.testcase_id}`;
    } else if (config.is_adhoc) {
      screenshotPath += `/${config.testcase_id}`;
    }
    screenshotPath += "/screenshot-" + timeStamp + ".png";
    try {
      if (!page.isClosed()) {
        const buffer = await page.screenshot();
        const data = {
          screenshot: buffer.toString("base64"),
          screenshotPath: screenshotPath,
          execution_id: config.execution_id,
        };

        const url = `${this.baseUrl}/organisations/${config.org_id}/projects/${config.project_id}/apps/${config.app_id}/${config.testlab}/${config.execution_id}/take-screenshot`;
        const res = await fetch(url, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify(data),
        });
      }
    } catch (error) {
      console.log("error while taking screenshot", error);
    }

    return screenshotPath;
  }

  public async updateStepCount(config: Config) {
    config.step_count = config.step_count + 1;
    const url = `${this.baseUrl}/organisations/${config.org_id}/projects/${config.project_id}/apps/${config.app_id}/${this.testlab}/${config.execution_id}/update-stepcount`;

    console.log(`${this.machineId} step_count ${config.step_count}`);
    return fetch(url, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        ...config,
        step_count: `${config.step_count}`,
        machine_id: this.machineId,
      }),
    });
  }

  public async postSessionDetails(page: Page, config: Config) {
    console.log(
      `Before tests ${config.execution_id} ${config.testcase_id} ${config.testsuite_id} ${config.testplan_id} ${config.is_adhoc} ${config.is_prerequisite} ${config.parent_testcase_id}`
    );
    config.testlab = this.testlab;
    this.config = config;

    let resp: any = {};

    try {
      resp = { ...resp, ...config };
      resp.machine_id = this.machineId;
      resp.testlab = this.testlab;
      resp.command_running = true;
      resp.step_count = "0";
      resp.status = "running";
      resp.duration = 0;
    } catch (error) {
      console.log("error while setting test case id and exeuction id", error);
    }
    const url = `${this.baseUrl}/organisations/${config.org_id}/projects/${config.project_id}/apps/${config.app_id}/${this.testlab}/sessions`;

    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(resp),
    });
    if (res.status == 200) {
      console.log(`testcaseid : ${config.testcase_id}, status : running`);
    }
  }

  public async updateSessionDetails(config: Config) {
    let resp: any = {};

    try {
      resp = { ...resp, ...config };
      resp.step_count = `${config.step_count}`;
    } catch (error) {
      console.log(
        "error while setting update session details response body",
        error
      );
    }
    const url = `${this.baseUrl}/organisations/${config.org_id}/projects/${config.project_id}/apps/${config.app_id}/${this.testlab}/sessions`;

    const res = await fetch(url, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(resp),
    });
  }

  public async uploadScreenshots(config: Config) {
    if (config.screenshot_type == "disabled") {
      return;
    }

    if (config.screenshot_type == "error-only" && config.status != "failed") {
      return;
    }

    const data: any = { ...config };
    data.machine_id = this.machineId;
    console.log("uploading screenshots");
    const url = `${this.baseUrl}/organisations/${config.org_id}/projects/${config.project_id}/apps/${config.app_id}/${config.testlab}/${config.execution_id}/upload-screenshots`;

    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(data),
    });
  }

  public async updateExecutionStatus(
    page: Page,
    config: Config,
    reason: string
  ) {
    this.isStatusUpdated = true;
    console.log(
      `In updateExecutionStatus ${this.machineId} ${config.execution_id} ${config.testcase_id} ${config.testsuite_id} ${config.testplan_id} ${config.is_adhoc} ${config.step_count} ${config.status} ${reason} ${this.testlab}`
    );
    try {
      if (!config.is_prerequisite) {
        await this.updateStatus(config, reason);
      }
    } catch (e) {
      console.log("failed to update test lab status due to ", e);
    }
  }

  public async postNetWorkLogs(config: Config, testInfo: any) {
    const url = `${this.baseUrl}/organisations/${config.org_id}/projects/${config.project_id}/apps/${config.app_id}/local-agent/network-logs`;
    const data: any = { ...config };
    data.machine_id = this.machineId;
    data.file_name = testInfo._;
    data.step_count = `${config.step_count}`;
    data.output_dir = testInfo.outputDir;
    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(data),
    });
  }

  public async updateStatus(config: Config, reason: string) {
    const testlab = config.testlab;
    const url = `${this.baseUrl}/organisations/${config.org_id}/projects/${config.project_id}/apps/${config.app_id}/${testlab}/${config.execution_id}/update-status`;
    const data: any = { ...config };
    data.message = reason;
    data.machine_id = this.machineId;
    data.command_running = true;

    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(data),
    });

    if (res.status == 200) {
      console.log(
        `testcaseid : ${config.testcase_id}, status : ${config.status}`
      );
    }
  }

    public formatDate(inputDate: string | Date, format = "YYYY-MM-DD"): string {
    if (!inputDate) {
      return ""; // might be intentionally left blank
    }

    let date: Date;
    let skipTimezoneConversion = false;

    if (typeof inputDate === "string") {
      // Handle different formats manually
      if (inputDate.includes("-")) {
        const parts = inputDate.split("-");
        if (parts.length === 3 && parts[2].length === 4) {
          // Assume DD-MM-YYYY and convert it
          skipTimezoneConversion = true;
          const [day, month, year] = parts.map(Number);
          date = new Date(year, month - 1, day);
        } else {
          // YYYY-MM-DD format
          date = new Date(inputDate);
        }
      } else {
        date = new Date(inputDate); // Try parsing directly
      }
    } else if (typeof inputDate === "number") {
      date = new Date(inputDate);
    } else {
      date = inputDate;
    }
    if (isNaN(date.getTime())) {
      throw new Error(`Generated Incorrect Date ${inputDate}`);
      // return inputDate as string; // Handle invalid input
    }
    if(!skipTimezoneConversion){
    date = this.changeTimezone(date, this.timezone);
    }

    const day = String(date.getDate()).padStart(2, "0");
    const month = String(date.getMonth() + 1).padStart(2, "0"); // Months are 0-indexed
    const year = date.getFullYear();
    console.log("format", format);
    if (format === "YYYY-MM-DD") {
      return `${year}-${month}-${day}`;
    }

    if (format === "DD-MMM-YYYY") {
      let shortMonth = date.toLocaleString("default", { month: "short" });
      return `${day}-${shortMonth}-${year}`;
    }

    return `${day}-${month}-${year}`;
  }

  public async fillDate(
    page: Page,
    xpath: string,
    date: string,
    format: string = "DD-MM-YYYY"
  ) {
    //TODO
    if (!xpath) {
      console.log(`empty selector in fill date`);
      return;
    }
    let formattedDate = date;
    if (date.includes("new Date")) {
      console.log("evaluating the date");
      formattedDate = eval(date);
      console.log(formattedDate);
    }
    if (!date.includes("?")) {
      formattedDate = this.formatDate(formattedDate, format);
    }
    await this.elementExists(page, xpath);

    await page.locator(xpath).focus();

    await page.evaluate(
      async ({ xpath, date }) => {
        const element = document.evaluate(
          xpath,
          document,
          null,
          XPathResult.FIRST_ORDERED_NODE_TYPE,
          null
        ).singleNodeValue;
        if (element instanceof HTMLInputElement) {
          element.setAttribute("value", date);
          // Dispatch 'input' and 'change' events
          element.dispatchEvent(new Event("change", { bubbles: true }));
        }
      },
      { xpath, date: formattedDate }
    );
    await page.waitForTimeout(3000);
    await this.veevaBlur(page, xpath);
  }

  public async clickSubmitButton(page: Page, xpath: string) {
    if (this.config.source === "EDC") {
      // await this.edc.blurAllElements(page, ".rowCtrlContainer");
      // await page.waitForTimeout(2000);
      await this.edc.submitForm(this.edcFormDetails);
      await page.reload();
      await page.waitForLoadState("domcontentloaded");
      await page.waitForTimeout(10000);
    } else {
      await page.locator(xpath).dispatchEvent("mousedown");
      await page.locator(xpath).dispatchEvent("mouseup");
      await page.locator(xpath).click();
    }
  }

  public async veevaClick(page: Page, xpath: string) {
    //TODO
    if (!xpath) {
      console.log(`empty selector in veeva click`);
      return;
    }
    await this.elementExists(page, xpath);
    await page.locator(xpath).dispatchEvent("click");
    await page.locator(xpath).focus();
    await this.veevaBlur(page, xpath);
  }

  public async veevaClickRadio(page: Page, xpath: string) {
    //TODO
    if (!xpath) {
      console.log(`empty selector in veeva click`);
      return;
    }
    try {
      if (
        xpath.includes("Arm 1") ||
        xpath.includes("Arm 2") ||
        xpath.includes("Arm 3")
      ) {
        try {
          await page.waitForSelector(xpath, {
            timeout: 3000,
          });
        } catch (e) {
          console.log(
            `Element not found for xpath ${xpath}, trying with Arm 1`
          );
          return;
        }
      } else {
        await this.elementExists(page, xpath);
      }

      await page.locator(xpath).focus();
      await page.locator(xpath).dispatchEvent("click");
      await this.veevaBlur(page, xpath);
    } catch (e) {
      console.log(e);
    }
  }

  public async veevaFill(page: Page, xpath: string, value: string) {
    //TODO
    if (!xpath) {
      console.log(`empty selector in veeva fill`);
      return;
    }
    await this.elementExists(page, xpath);
    //TODO: Currently assuming if value contains new Date() as time
    if (value.includes("new Date")) {
      let result = eval(value);
      if (typeof result === "number") {
        let dateTime = new Date(result);
        dateTime = this.changeTimezone(dateTime, this.timezone);
        value = `${String(dateTime.getHours()).padStart(2, "0")}:${String(
          dateTime.getMinutes()
        ).padStart(2, "0")}`;
      } else if (result instanceof Date) {
        result = this.changeTimezone(result, this.timezone);
        value = `${String(result.getHours()).padStart(2, "0")}:${String(
          result.getMinutes()
        ).padStart(2, "0")}`;
      }
    } else if (value.includes(":")) {
      let arr = value.split(":");
      if (arr.length === 2) {
        let dateTime = new Date();
        dateTime.setHours(parseInt(arr[0]));
        dateTime.setMinutes(parseInt(arr[1]));
        dateTime = this.changeTimezone(dateTime, this.timezone);
        value = `${String(dateTime.getHours()).padStart(2, "0")}:${String(
          dateTime.getMinutes()
        ).padStart(2, "0")}`;
      }
    }
    await page.fill(xpath, value);
    await page.waitForTimeout(2000);
    await this.veevaBlur(page, xpath);
  }

  normalizeSpace(str: string) {
    return str.replace(/\s+/g, " ").trim();
  }

  public async veevaDialogAssert(
    page: Page,
    dialogXPath: string,
    xpath: string,
    value: string,
    isPositive: boolean
  ) {
    const timeout = isPositive ? 180000 : 30000;
    try {
      await this.elementExists(page, dialogXPath, timeout);
    } catch {}
    const locator = page.locator(dialogXPath);
    console.log(`after dialog locator`);
    const count = await locator.count(); // Get the total number of matching elements
    console.log(`dialog count ${count}`);
    if (isPositive) {
      if (count == 0) {
        throw new Error(
          `Cannot assert value, no elements found with xpath ${dialogXPath}`
        );
      }
    }

    if (count === 1) {
      await page.click(dialogXPath);
      await page.waitForLoadState("domcontentloaded");
      await page.waitForTimeout(2000);
      await this.veevaAssert(page, xpath, value, isPositive);
    }
  }

  public async veevaAssert(
    page: Page,
    xpath: string,
    value: string,
    isPositive: boolean
  ) {
    //TODO
    if (!xpath) {
      console.log(`empty selector in veeva assert`);
      return;
    }
    if (value) {
      console.log(`original value ${value}`);
      value = value.replace(/\\/g, "");
      console.log(`cleaned value ${value}`);
    }
    const timeout = isPositive ? 180000 : 30000;
    try {
      await this.elementExists(page, xpath, timeout);
    } catch {}
    console.log(`before locator ${xpath}`);
    const locator = page.locator(xpath);
    console.log(`after locator`);
    const count = await locator.count(); // Get the total number of matching elements
    console.log(`count ${count}`);
    if (isPositive) {
      // Check that at least one element contains the text
      let found = false;

      if (count === 0) {
        throw new Error(
          `Cannot assert value, no elements found with xpath ${xpath}`
        );
      }

      for (let i = 0; i < count; i++) {
        let text = await locator.nth(i).textContent();
        console.log(`element value ${text}`);
        if (text) {
          text = text.toLowerCase();
          text = this.normalizeSpace(text);
          value = value.toLowerCase();
          value = this.normalizeSpace(value);
          if (text.includes(value)) {
            found = true;
            break;
          }
        }
      }

      if (!found) {
        throw new Error(
          `Assert Failed due to value not found: ${value} in element ${xpath}`
        );
      }
    } else {
      // Assert that none of the elements contain the text
      for (let i = 0; i < count; i++) {
        let text = await locator.nth(i).textContent();
        if (text) {
          text = text.toLowerCase();
          text = this.normalizeSpace(text);
          value = value.toLowerCase();
          value = this.normalizeSpace(value);
          if (text.includes(value)) {
            throw new Error(
              `Assert Failed due to value found: ${value} in element ${xpath}`
            );
          }
        }
      }
    }
  }

  public async veevaBlur(page: Page, childXpath: string) {
    // (//*[@selname='VISDAT']//input[@placeholder='date'])[1]/ancestor::*[contains(@class, "rowCtrlContainer")]

    // const parentSelector =
    //   childXpath + "/ancestor::*[contains(@class, 'rowCtrlContainer')]";
    // await page.locator(parentSelector).dispatchEvent("blur");
    await page.keyboard.press("Tab");
    if (!this.edcFormDetails.resetForm) {
      await page.keyboard.press("Tab");
    }
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(4000);
  }

  public async addItemGroup(page: Page, itemGroupName: string) {
    if (this.config.source === "EDC") {
      await this.edc.addItemGroup(itemGroupName, this.edcFormDetails);
      await page.reload();
      await page.waitForLoadState();
    }
  }

  public async addNewSection(page: Page, newSection: string) {
    if (this.config.source === "EDC") {
      let formRepeatSequence = 1;
      const splitArr = newSection.split(":");
      let eventGroupId = "",
        eventId = "",
        formId = "",
        itemGroupName = "";
      if (splitArr.length == 4) {
        [eventGroupId, eventId, formId, itemGroupName] = splitArr;
      } else if (splitArr.length == 5) {
        eventGroupId = splitArr[0];
        eventId = splitArr[1];
        formId = splitArr[2];
        itemGroupName = splitArr[3];
        formRepeatSequence = parseInt(splitArr[4]);
      }
      // check whether event exists
      await this.edc.createEventIfNotExists(eventGroupId, eventId);

      const isItemGroupCreated = await this.edc.addItemGroup(itemGroupName, {
        eventGroupId,
        eventId,
        formId,
        formRepeatSequence,
      });
      if (isItemGroupCreated) {
        await page.reload();
        await page.waitForLoadState("domcontentloaded");
        await page.waitForTimeout(2000);
      }
    }
  }

  public async editForm(page: Page) {
    await Promise.race([
      page.waitForSelector("//*[contains(text(),'Edit Form')]", {
        state: "visible",
      }), // Replace with your actual selector for the Edit button
      page.waitForSelector("//*[contains(text(),'Submit')]", {
        state: "visible",
      }), // Replace with your actual selector for the Submit button
    ]);

    const editButton = page.locator("//*[contains(text(),'Edit Form')]");

    if ((await editButton.count()) > 0) {
      console.log(`edit button ${editButton}`);

      await page
        .locator("//*[contains(text(),'Edit Form')]")
        .dispatchEvent("click");
      await page.waitForLoadState("networkidle");
      await page.waitForLoadState("domcontentloaded");
      await page.waitForTimeout(2000);

      await page
        .locator(`//*[@role="dialog"]//*[3]/input`)
        .dispatchEvent("click");
      await page.waitForTimeout(2000);

      await page
        .locator("//li/a[contains(text(),'Self-evident correction')]")
        .dispatchEvent("click");
      await page
        .locator("//*[contains(text(),'Continue')]")
        .dispatchEvent("click");
      return true;
    }
    return false;
  }

  public async resetForm(
    page: Page,
    resetForm: boolean = true,
    isSubjectNumberForm = false
  ) {
    const isEdited = await this.editForm(page);
    if (isEdited) {
      await page.waitForLoadState("networkidle");
      await page.waitForLoadState("domcontentloaded");
      await page.waitForSelector(`//div[contains(@class,'vdc_loading')]`, {
        state: "hidden",
      });
      await page.waitForTimeout(2000);
    }
    // Subject Number Form
    if (isSubjectNumberForm) {
      let subjectNumberSelector =
        "(//*[@selname='SUBJID']//div[@data-item-label='Subject Number [xxxxx]']//input)[1]";
      try {
        await this.elementExists(page, subjectNumberSelector, 30000);
        let subjectNumber = this.config.VAULT_SUBJECT_NAME;
        if (
          this.parallelismEnabled &&
          this.edcSubjectDetails &&
          this.workerIndex < this.edcSubjectDetails.subject_numbers.length
        ) {
          subjectNumber =
            this.edcSubjectDetails.subject_numbers[this.workerIndex];
        }
        await this.veevaFill(page, subjectNumberSelector, subjectNumber);
      } catch {}
      return;
    }

    if (!resetForm) {
      return;
    }
    await page
      .locator(
        "//*[@class='vv-cdm-dropdownmenu cdm-asyncdropdownmenu cdm-form-more-actions css-1fbcgml-DropdownMenu']/button"
      )
      .dispatchEvent("click");

    await page.waitForLoadState("networkidle");
    await page.waitForLoadState("domcontentloaded");
    await page.waitForTimeout(2000);

    if (isEdited) {
      await page.waitForSelector("//*[contains(text(),'Reset Form')]", {
        state: "visible",
      });
    }

    const resetButton = page.locator("//*[contains(text(),'Reset Form')]");

    if ((await resetButton.count()) > 0) {
      console.log(`reset button ${resetButton}`);
      await page
        .locator("//*[contains(text(),'Reset Form')]")
        .dispatchEvent("click");
      await page.waitForLoadState("domcontentloaded");
      await page.fill(
        "//div[contains(@class,'vv-cdm-dialog')]//*[@class='css-ew23ed-Input']",
        "RESET"
      );
      await page.locator("//*[@title='Reset']").dispatchEvent("click");
      await page.waitForLoadState("networkidle");
      await page.waitForLoadState("domcontentloaded");
      await page.waitForSelector(`//div[contains(@class,'vdc_loading')]`, {
        state: "hidden",
      });
      await page.waitForTimeout(5000);
    }
  }

  public async markAsBlank(page: Page) {
    await this.resetForm(page);

    const blankButton = page.locator(`//button[text()="Mark as Blank"]`);

    if ((await blankButton.count()) > 0) {
      console.log(`blank button ${blankButton}`);
      await blankButton.dispatchEvent("click");
      await page.waitForLoadState("domcontentloaded");
      await page
        .locator("//div[contains(@class,'vv-cdm-dialog')]//input")
        .focus();
      await page
        .locator(
          "//div[contains(@class,'vv-cdm-overlay vv-cdm-select-menu')]//li[1]"
        )
        .dispatchEvent("click");
      await page
        .locator(
          `//div[contains(@class,'vv-cdm-dialog')]//button[contains(text(),'Submit')]`
        )
        .dispatchEvent("click");
      await page.waitForLoadState("networkidle");
      await page.waitForLoadState("domcontentloaded");
      await page.waitForTimeout(5000);
    }
  }

  public async uploadVideo(config: Config, testInfo: any) {
    const testlab = config.testlab;
    const url = `${this.baseUrl}/organisations/${config.org_id}/projects/${config.project_id}/apps/${config.app_id}/${config.testlab}/${config.execution_id}/upload-video`;
    const data: any = { ...config };
    data.machine_id = this.machineId;
    data.step_count = `${config.step_count}`;
    data.output_dir = testInfo.outputDir;
    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(data),
    });
  }
  public async Locator(page: Page, selector: String) {
    let parent: any = page;
    while (true) {
      let canBreak = true;
      if (selector.indexOf("<ECL_IFRAME>") !== -1) {
        const parentSelector = selector.slice(
          0,
          selector.indexOf("<ECL_IFRAME>")
        );
        parent = parent.frameLocator(parentSelector);
        selector = selector.slice(selector.indexOf("<ECL_IFRAME>") + 12);
        canBreak = false;
      }
      //TODO handle shadow dom
      if (canBreak) {
        return parent.locator(selector);
      }
    }
  }

  public async postStep(testpage: Page) {
    await testpage.waitForLoadState();

    let mainFrame = testpage.mainFrame();

    let promises: Promise<void>[] = [];

    console.log(`child frames length:  ${mainFrame.childFrames().length}`);

    for (const child of mainFrame.childFrames()) {
      promises.push(child.waitForLoadState());
    }

    await Promise.all(promises);
  }

  public async veevaAssertAction({
    Expectation,
    Action,
    eventName,
    formName,
    eventGroupName,
  }: {
    Expectation: boolean;
    Action: string;
    eventName: string;
    formName: string;
    eventGroupName: string;
  }) {
    await this.edc.AssertEventOrForm({
      Expectation,
      Action,
      eventName,
      formName,
      eventGroupName,
    });
  }

  public async fillEventDate(
    eventGroupName: string,
    eventName: string,
    eventDate: string
  ) {
    if (eventDate.includes("new Date")) {
      eventDate = eval(eventDate);
    }
    const formattedDate = this.formatDate(eventDate, "YYYY-MM-DD");

    await this.edc.createEventIfNotExists(
      eventGroupName,
      eventName,
      formattedDate,
      true
    );
  }

  public async fillEventsDate(data: string) {
    await this.edc.setEventsDate(data);
  }

  public async setEventDidNotOccur(
    eventGroupName: string,
    eventName: string,
    eventDate: string
  ) {
    await this.edc.setEventDidNotOccur(eventGroupName, eventName, eventDate);
  }

  public async setEventsDidNotOccur(data: string) {
    await this.edc.setEventsDidNotOccur(data);
  }

  public async assertUrl(page: Page, url: string) {
    await expect(page).toHaveURL(url);
  }

  public async assertUrlNotMatch(page: Page, url: string) {
    await expect(page).not.toHaveURL(url);
  }

  public async assertText(page: Page, xpath: string, text: string) {
    await expect(page.locator(xpath)).toContainText(text);
  }

  public async assertTextNotContain(page: Page, xpath: string, text: string) {
    await expect(page.locator(xpath)).not.toContainText(text);
  }

  public async assertVisible(page: Page, xpath: string) {
    await expect(page.locator(xpath)).toBeVisible();
  }

  public async assertNotVisible(page: Page, xpath: string) {
    await expect(page.locator(xpath)).not.toBeVisible();
  }

  public async assertValue(page: Page, xpath: string, value: string) {
    await expect(page.locator(xpath)).toHaveValue(value);
  }

  public async assertValueAbsent(page: Page, xpath: string, value: string) {
    await expect(page.locator(xpath)).not.toHaveValue(value);
  }

  public async assertChecked(page: Page, xpath: string) {
    await expect(page.locator(xpath)).toBeChecked();
  }

  public async assertNotChecked(page: Page, xpath: string) {
    await expect(page.locator(xpath)).not.toBeChecked();
  }

  // public async elementExists(page: Page, selector: string, timeout = 180000) {
  //   try {
  //     await page.waitForSelector(selector, { timeout, state: "attached" });
  //   } catch (error) {
  //     throw new Error(`Element not found with locator ${selector}.`);
  //   }
  //   const count = await page.locator(selector).count();
  //   if (count > 1) {
  //     throw new Error(`Multiple Element Found for ${selector}.`);
  //   }
  // }

  public async elementExists(page: Page, selector: string, timeout = 10000): Promise<boolean> {
  try {
    await page.waitForSelector(selector, { timeout, state: "attached" });
    const count = await page.locator(selector).count();

    if (count > 1) {
      console.warn(`⚠️ Multiple elements found for selector: ${selector}`);
    }

    return count > 0;
  } catch (error) {
    console.warn(`❌ Element not found for selector: ${selector} within ${timeout}ms`);
    if(selector== "(//*[@selname='LBCTEST_UMICRO']//*[normalize-space(text())='Ammonium Biurate Crystals'])[1]"){
      console.log("selector is LBCTEST_UMICRO");
      const buttonCount = await this.elementExists(page, "/html/body/div[2]/div[2]/div[1]/div/div/div/div[2]/div[6]/div/div/div/div[2]/div/div/div/div/div/div[2]/div/div[2]/div/div[1]/div[2]/div/form/div[4]")
      console.log("button count is:",buttonCount);
      if(buttonCount){
        console.log("button is visible");
        await page.locator("/html/body/div[2]/div[2]/div[1]/div/div/div/div[2]/div[6]/div/div/div/div[2]/div/div/div/div/div/div[2]/div/div[2]/div/div[1]/div[2]/div/form/div[4]").dispatchEvent("click");
        console.log("button clicked");
      }
      if(await this.elementExists(page,selector)){
        return true;
      }
    }
    return false;
  }
}


  private extractTimezone(timezoneStr: string) {
    const match = timezoneStr.match(/\(([^)]+\/[^)]+)\)$/);
    if (!match || match.length < 2) {
      throw new Error("Invalid timezone format");
    }

    return match[1];
  }

  public changeTimezone(date: Date, timezone: string) {
    return new Date(
      date.toLocaleString("en-US", {
        timeZone: timezone,
      })
    );
  }
}

export const test = base.extend<{
  utils: Utils;
  saveStatus: void;
  forEachTest: void;
}>({
  utils: async ({}, use, testInfo) => {
    const utilsObject = new Utils();
    utilsObject.machineId = testInfo.project.name;
    utilsObject.parallelismEnabled = process.env.PARALLELISM_ENABLED == "true";
    console.log(`parallelism enabled ${process.env.PARALLELISM_ENABLED}`);
    utilsObject.edcSubjectDetails = edcDetails;
    console.log(`utils created for machine ${testInfo.project.name}`);
    await use(utilsObject);
  },

  saveStatus: [
    async ({ utils }, use, testInfo) => {
      await use();
      if (!utils.isStatusUpdated) {
        console.log(
          `save status ${testInfo.testId} ${testInfo.project.name} ${testInfo.title} ${testInfo.status} ${testInfo.expectedStatus}`
        );
        const config = utils.config;
        if (config) {
          config.status = "failed";

          console.log(
            `save status ${testInfo.testId} ${testInfo.project.name} ${testInfo.title} ${testInfo.status} ${testInfo.expectedStatus}`
          );
          await utils.uploadScreenshots(config);
          await utils.updateStatus(config, testInfo.error?.message || "");
        }
      }
    },
    { auto: true },
  ],
  forEachTest: [
    async ({ page, utils }, use, testInfo) => {
      await use();
      const config = utils.config;
      if (config) {
        let duration = 0;
        duration = testInfo.duration / 1000;
        config.duration = Math.ceil(duration);
        await utils.updateSessionDetails(config);
        console.log("uploading network logs of testcase");
        await utils.postNetWorkLogs(config, testInfo);
        console.log("uploading video of testcase");
        await utils.uploadVideo(config, testInfo);
      }
    },
    { auto: true },
  ],
});
