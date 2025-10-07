import { Page } from "@playwright/test";
import { Agent, fetch, setGlobalDispatcher } from "undici";
setGlobalDispatcher(new Agent({ connect: { timeout: 60_000 } }));

declare var process: {
  env: {
    TESTLAB: string;
    BASE_PATH: string;
    ELEMENT_TIMEOUT: string;
  };
};

interface ECL_UTILS {
  formsReset: string[];
  resetForm(page: Page): Promise<void>;
  formatDate(inputDate: string | Date, format?: string): string;
}

export default class EDC {
  vaultDNS: string;
  version: string;
  studyName: string;
  studyCountry: string;
  siteName: string;
  subjectName: string;
  sessionId: string;
  vaultOrigin: string;
  utils: ECL_UTILS;

  constructor({
    vaultDNS,
    version,
    studyName,
    studyCountry,
    siteName,
    subjectName,
    utils,
  }: {
    vaultDNS: string;
    version: string;
    studyName: string;
    studyCountry: string;
    siteName: string;
    subjectName: string;
    utils: ECL_UTILS;
  }) {
    this.vaultDNS = vaultDNS;
    this.version = version;
    this.studyName = studyName;
    this.studyCountry = studyCountry;
    this.siteName = siteName;
    this.subjectName = subjectName;
    this.sessionId = "";
    this.vaultOrigin = "";
    this.utils = utils;
  }

  async authenticate(userName: string, password: string) {
    const url = `https://${this.vaultDNS}/api/${this.version}/auth`;
    const headers = {
      "Content-Type": "application/x-www-form-urlencoded",
      Accept: "application/json",
    };

    const body = new URLSearchParams({
      username: userName,
      password: password,
    });

    try {
      const response = await fetch(url, {
        method: "POST",
        body: body,
        headers: headers,
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data: any = await response.json();
      console.log("Response:", data);

      const vaultId = data.vaultId;
      const vaults = data.vaultIds;
      let vaultMatch = false;

      if (vaults != null) {
        for (var i = 0; i < vaults.length; i++) {
          var vault = vaults[i];

          if (vault.id == vaultId) {
            this.sessionId = data.sessionId;
            vaultMatch = true;

            const parsedUrl = new URL(vault.url);
            this.vaultOrigin = parsedUrl.origin;

            break;
          }
        }
      }
      return vaultMatch;
    } catch (e) {
      console.error("Error:", e);
      return false;
    }
  }

  async getSiteDetails() {
    if (this.sessionId == null) {
      return "";
    }

    //TODO: Need to handle pagination
    try {
      const response = await fetch(
        `https://${this.vaultDNS}/api/${this.version}/app/cdm/sites?study_name=${this.studyName}`,
        {
          method: "GET",
          headers: {
            Authorization: `Bearer ${this.sessionId}`,
          },
        }
      );

      const data: any = await response.json();

      const sites = data.sites;
      let siteDetails: any;
      sites.forEach((site: any) => {
        if (site.site === this.siteName) {
          siteDetails = site;
          return;
        }
      });

      return siteDetails;
    } catch (e) {
      console.error("Error:", e);
      return "";
    }
  }

  async getSubjectNavigationURL() {
    if (this.sessionId == null) {
      return "";
    }

    try {
      const response = await fetch(
        `https://${this.vaultDNS}/api/${this.version}/app/cdm/subjects?study_name=${this.studyName}&site=${this.siteName}`,
        {
          method: "GET",
          headers: {
            Authorization: `Bearer ${this.sessionId}`,
          },
        }
      );

      const data: any = await response.json();

      const subjects = data.subjects;
      let cdms_url: string = "";
      subjects.forEach((subject: any) => {
        if (
          subject.study_name === this.studyName &&
          subject.site === this.siteName &&
          subject.subject === this.subjectName
        ) {
          cdms_url = subject.cdms_url;
          return;
        }
      });

      return `${this.vaultOrigin}${cdms_url}`;
    } catch (e) {
      console.error("Error:", e);
      return "";
    }
  }

  getCurrentDateFormatted() {
    const now = new Date();
    const year = now.getFullYear();
    const month = String(now.getMonth() + 1).padStart(2, "0"); // Months are 0-indexed
    const day = String(now.getDate()).padStart(2, "0");

    return `${year}-${month}-${day}`;
  }

  async createEventIfNotExists(
    eventGroupName: string,
    eventName: string,
    eventDate: string = this.getCurrentDateFormatted(),
    replaceDate: boolean = false
  ): Promise<boolean> {
    if (this.sessionId == null) {
      return false;
    }

    try {
      // check whether event exists
      let { eventExists, response, eventDatePresent } =
        await this.checkIfEventExists(eventName, eventGroupName);

      console.log("eventExists", eventExists);
      console.log("eventDatePresent", eventDatePresent);
      if (!eventExists) {
        // create event group and event
        await this.createEventGroup(eventGroupName, response, eventDate);
        await this.setEventDate(eventGroupName, eventName, eventDate);
      }

      if (eventExists && (replaceDate || !eventDatePresent)) {
        await this.setEventDate(eventGroupName, eventName, eventDate);
        eventExists = false; // this is to reload the page after setting the event date
      }

      return eventExists;
    } catch (e) {
      console.error("Error:", e);
      throw new Error(`Unable to create event for ${eventName} due to ${e}`);
    }
  }

  async setEventDidNotOccur(
    eventGroupName: string,
    eventName: string,
    eventDate: string
  ) {
    if (this.sessionId == null) {
      return false;
    }

    try {
      const response = await fetch(
        `https://${this.vaultDNS}/api/${this.version}/app/cdm/events/actions/didnotoccur`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${this.sessionId}`,
            Accept: "application/json",
          },
          body: JSON.stringify({
            study_name: this.studyName,
            events: [
              {
                study_country: this.studyCountry,
                site: this.siteName,
                subject: this.subjectName,
                eventgroup_name: eventGroupName,
                event_name: eventName,
                change_reason: "missed visit",
              },
            ],
          }),
        }
      );

      const responseData: any = await response.json();
      if (
        responseData &&
        responseData.responseStatus.toLowerCase() === "success" &&
        responseData.events[0].responseStatus.toLowerCase() === "success"
      ) {
        return true;
      }
      console.log(responseData);
      throw new Error("Failed to set event did not occur");
    } catch (e) {
      console.error(
        `Error: Unable to set event did not occur for ${eventName} due to ${e}`
      );
      throw new Error(
        `Unable to set event did not occur for ${eventName} due to ${e}`
      );
    }
  }

  private async checkIfEventExists(eventName: string, eventGroupName: string) {
    let data: any, response: any;
    try {
      response = await fetch(
        `https://${this.vaultDNS}/api/${this.version}/app/cdm/events?study_name=${this.studyName}&study_country=${this.studyCountry}&site=${this.siteName}&subject=${this.subjectName}`,
        {
          method: "GET",
          headers: {
            Authorization: `Bearer ${this.sessionId}`,
          },
        }
      );
      data = await response.json();
    } catch (e) {
      throw new Error(`Get Event API Failed due to ${e}`);
    }

    console.log("events response");
    console.log(data);

    const events = data.events;
    let eventExists: boolean = false;
    let eventDatePresent: boolean = false;
    events.forEach((event: any) => {
      if (
        event.study_country === this.studyCountry &&
        event.site === this.siteName &&
        event.subject === this.subjectName &&
        event.event_name === eventName &&
        event.eventgroup_name === eventGroupName
      ) {
        eventExists = true;
        if (event.event_date) {
          eventDatePresent = true;
        }
        return;
      }
    });
    return { eventExists, response, eventDatePresent };
  }

  private async createEventGroup(
    eventGroupName: string,
    response: Response,
    eventDate: string
  ) {
    const createEGResponse = await fetch(
      `https://${this.vaultDNS}/api/${this.version}/app/cdm/eventgroups`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${this.sessionId}`,
        },
        body: JSON.stringify({
          study_name: this.studyName,
          eventgroups: [
            {
              study_country: this.studyCountry,
              site: this.siteName,
              subject: this.subjectName,
              eventgroup_name: eventGroupName,
              date: eventDate,
            },
          ],
        }),
      }
    );

    if (!createEGResponse.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const respJson: any = await createEGResponse.json();
    if (respJson.responseStatus != "SUCCESS") {
      throw new Error(respJson.responseMessage);
    }

    console.log("respJson", respJson);
  }

  private async setEventDate(
    eventGroupName: string,
    eventName: string,
    eventDate: string = this.getCurrentDateFormatted()
  ) {
    const setEVDateResp = await fetch(
      `https://${this.vaultDNS}/api/${this.version}/app/cdm/events/actions/setdate`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${this.sessionId}`,
          Accept: "application/json",
        },
        body: JSON.stringify({
          study_name: this.studyName,
          events: [
            {
              study_country: this.studyCountry,
              site: this.siteName,
              subject: this.subjectName,
              eventgroup_name: eventGroupName,
              event_name: eventName,
              date: eventDate,
            },
          ],
        }),
      }
    );

    if (!setEVDateResp.ok) {
      throw new Error(`HTTP error! status: ${setEVDateResp.status}`);
    }

    const setEVDateRespJson: any = await setEVDateResp.json();
    if (setEVDateRespJson.responseStatus != "SUCCESS") {
      throw new Error(setEVDateRespJson.responseMessage);
    }

    console.log("setEVDateRespJson", setEVDateRespJson);
  }

  async setEventsDate(data: string) {
    if (this.sessionId == null) {
      throw new Error("Session ID is null");
    }
    const events: any = [];
    const arr = data.split(",");
    for (let i = 0; i < arr.length; i++) {
      const [eventInfo, value] = arr[i].split("=");
      let [eventGroupName, eventName] = eventInfo.split(":");
      let eventDate: any = value.trim();
      if (value.includes("new Date")) {
        eventDate = eval(eventDate);
      }
      const formattedDate = this.utils.formatDate(eventDate, "YYYY-MM-DD");
      eventGroupName = eventGroupName.trim();
      eventName = eventName.trim();
      events.push({
        study_country: this.studyCountry,
        site: this.siteName,
        subject: this.subjectName,
        eventgroup_name: eventGroupName,
        event_name: eventName,
        date: formattedDate,
      });
    }
    // Add limit of 100 events per request
    for (let i = 0; i < events.length; i += 100) {
      const eventsChunk = events.slice(i, i + 100);
      try {
        const setEVDateResp = await fetch(
          `https://${this.vaultDNS}/api/${this.version}/app/cdm/events/actions/setdate`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              Authorization: `Bearer ${this.sessionId}`,
              Accept: "application/json",
            },
            body: JSON.stringify({
              study_name: this.studyName,
              events: eventsChunk,
            }),
          }
        );

        if (!setEVDateResp.ok) {
          throw new Error(`HTTP error! status: ${setEVDateResp.status}`);
        }

        const setEVDateRespJson: any = await setEVDateResp.json();
        if (setEVDateRespJson.responseStatus != "SUCCESS") {
          throw new Error(setEVDateRespJson.responseMessage);
        }

        console.log("setEVDateRespJson", setEVDateRespJson);
      } catch (error) {
        console.error(`Error setting event date: ${error}`);
        throw new Error(`Error setting event date: ${error}`);
      }
    }
  }

  async setEventsDidNotOccur(data: string) {
    if (this.sessionId == null) {
      return false;
    }

    const events: any = [];
    const arr = data.split(",");
    for (let i = 0; i < arr.length; i++) {
      let [eventGroupName, eventName] = arr[i].split(":");
      eventGroupName = eventGroupName.trim();
      eventName = eventName.trim();
      events.push({
        study_country: this.studyCountry,
        site: this.siteName,
        subject: this.subjectName,
        eventgroup_name: eventGroupName,
        event_name: eventName,
        change_reason: "missed visit",
      });
    }
    // Add limit of 100 events per request
    for (let i = 0; i < events.length; i += 100) {
      const eventsChunk = events.slice(i, i + 100);
      try {
        const response = await fetch(
          `https://${this.vaultDNS}/api/${this.version}/app/cdm/events/actions/didnotoccur`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              Authorization: `Bearer ${this.sessionId}`,
              Accept: "application/json",
            },
            body: JSON.stringify({
              study_name: this.studyName,
              events: eventsChunk,
            }),
          }
        );

        const responseData: any = await response.json();
        if (
          responseData &&
          responseData.responseStatus.toLowerCase() === "success"
        ) {
          return true;
        }
        console.log(responseData);
        throw new Error("Failed to set event did not occur");
      } catch (e) {
        console.error(`Error: Unable to set event did not occur due to ${e}`);
        throw new Error(`Unable to set event did not occur for due to ${e}`);
      }
    }

    return true;
  }

  public async elementExists(
    page: Page,
    selector: string,
    timeout = Number(process.env.ELEMENT_TIMEOUT)
  ) {
    try {
      await page.waitForSelector(selector, { timeout, state: "attached" });
      return true;
    } catch (error) {
      return false;
    }
  }

  async resetStudyDrugAdministrationForms(page: Page) {
    const sideNavLocator = `(//li[@class='cdm-tree-item-node']//div[starts-with(@id, 'OPC')]//a[contains(translate(text(), 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'), 'study treatment administration - risankizumab arm')][ancestor::li[@class='cdm-tree-item-node']//div[starts-with(@id, 'OPS')]//a[contains(translate(text(), 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'), 'period 1 day 1')]])[1]`;
    const resetButtonLocator = `//div[contains(@class, "vdc_vertical_middle")]//button[contains(translate(., 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'), 'reset form')]`;
    const dialogButtonLocator = `//div[@role='dialog']//a[contains(translate(., 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'), 'reset')]`;
    try {
      await page.waitForSelector(sideNavLocator, {
        timeout: 3000,
      });
    } catch (e) {
      return;
    }
    const formLinks = await page.locator(sideNavLocator);
    const count = await formLinks.count();

    for (let i = 0; i < count; i++) {
      const formLink = formLinks.nth(i);

      await formLink.scrollIntoViewIfNeeded();

      await formLink.click();

      await page.waitForTimeout(1000);

      let resetButton;
      try {
        resetButton = await page.waitForSelector(resetButtonLocator, {
          timeout: 3000,
        });
      } catch (e) {
        return;
      }

      if (resetButton) {
        await resetButton.click();
        await page.waitForTimeout(1000);

        let dialogButton;
        try {
          dialogButton = await page.waitForSelector(dialogButtonLocator, {
            timeout: 3000,
          });
        } catch (e) {
          return;
        }
        if (dialogButton) {
          await dialogButton.click();
          await page.waitForTimeout(1000);
        }
      }
    }
  }

  async safeDispatchClick(page: Page, locator: string, {
  expectedSelector,
  maxRetries = 3,
  waitTimeout = 5000
}: {
  expectedSelector?: string; // something that should appear after click
  maxRetries?: number;
  waitTimeout?: number;
} = {}): Promise<boolean> {

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    console.log(`Attempt ${attempt}: Dispatching click on ${locator}`);

    // Track the current URL for navigation detection
    const urlBefore = page.url();

    // Perform click
    await page.locator(locator).dispatchEvent("click");

    let success = false;

    // Wait for either a DOM change or a navigation
    try {
      if (expectedSelector) {
        await page.locator(expectedSelector).waitFor({ timeout: waitTimeout });
        success = true;
      } else {
        // Fallback: check if URL changed
        await page.waitForFunction(
          (prevUrl) => window.location.href !== prevUrl,
          urlBefore,
          { timeout: waitTimeout }
        );
        success = true;
      }
    } catch {
      console.warn(`Click attempt ${attempt} did not trigger expected change`);
    }

    if (success) {
      console.log("Click successful");
      return true;
    }
  }

  console.error(`Failed to click ${locator} after ${maxRetries} attempts`);
  return false;
}

  async getFormLinkLocator({
    page,
    navigation_details,
  }: {
    page: Page;
    navigation_details: {
      formId: string;
      eventId: string;
      eventGroupId: string;
      formName: string;
      eventName: string;
      formRepeats: string;
      formRepeatMaxCount: number;
      formSequenceIndex: number;
      isRelatedToStudyTreatment?: boolean;
    };
  }): Promise<{
    locatorExists: boolean;
    eventExisted: boolean;
    error?: any;
  }> {
    if (this.sessionId == null) {
      return { locatorExists: false, eventExisted: true };
    }

    if (navigation_details.isRelatedToStudyTreatment) {
      await this.resetStudyDrugAdministrationForms(page);
    }

    try {
      let {
        formId,
        eventId,
        eventGroupId,
        formName,
        eventName,
        formRepeats,
        formRepeatMaxCount,
        formSequenceIndex = 1,
      } = navigation_details;

      console.log(navigation_details);

      // check whether event exists
      const eventExisted = await this.createEventIfNotExists(
        eventGroupId,
        eventId
      );

      console.log(`eventExisted: ${eventExisted}`);

      formName = formName.toLowerCase().replace(/\s+\(\d+\)$/, ""); // remove repeat number from form name
      eventName = eventName.toLowerCase();

      let sideNavLocator = `(//li[@class='cdm-tree-item-node']//div[starts-with(@id, 'OPC')]//a[contains(translate(text(), 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'), '${formName}')][ancestor::li[@class='cdm-tree-item-node']//div[starts-with(@id, 'OPS')]//a[contains(translate(text(), 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'), '${eventName}')]])[1]`;

      if (eventGroupId === "eg_COMMON" && eventId === "ev_COMMON") {
        sideNavLocator = `//div[@class="cdm-log-form-panel"]//a[contains(translate(text(), 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'), '${formName}')]`;
      }

      console.log(`form locator: ${sideNavLocator}`);

      if (!eventExisted) {
        await page.reload();
        await page.waitForLoadState("domcontentloaded");
      }
      const exits = await this.elementExists(page, sideNavLocator);
      if (!exits) {
        throw new Error(`${formName} form is not found`);
      }
      if (sideNavLocator.includes("study treatment administration - risankizumab arm")) {
        const expectedSelector = `//div[contains(@class,'vdc_title') and contains(translate(text(), 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'), '${formName.toLowerCase()}')]`;
        const clicked = await this.safeDispatchClick(page, sideNavLocator, {
          expectedSelector,
          maxRetries: 3,
          waitTimeout: 5000
        });
        if (!clicked) {
          throw new Error(`${formName} click failed after retries`);
        }
      }else{
        await page.locator(sideNavLocator).dispatchEvent("click");
      }
      console.log(
        "check if form can repeat",
        formRepeats.toLowerCase() === "yes" && formRepeatMaxCount >= 1
      );
      if (formRepeats.toLowerCase() === "yes" && formRepeatMaxCount >= 1) {
        console.log(`formName ${formName}`);
        console.log(`forms reset ${this.utils.formsReset}`);
        if (
          !this.utils.formsReset.includes(formName) &&
          formSequenceIndex === 1
        ) {
          let noRecordsLocator = `//div[contains(@class,"vdc_repeat_forms_page")]//div[contains(text(),"No records found")]`;
          const noRecordsExists = await this.elementExists(
            page,
            noRecordsLocator,
            15000
          );
          if (!noRecordsExists) {
            // reseting existing repeats
            let index = 0;
            let currentUrl = await page.url();
            while (true) {
              const repeatitiveFormLocator = `(//div[contains(@class,'vdc_repeat_forms_page')]//table[contains(@class, 'vv_row_hover')]/tbody/tr[td/div])[${
                index + 1
              }]`;
              const repeatitiveFormExists = await this.elementExists(
                page,
                repeatitiveFormLocator,
                15000
              );

              if (!repeatitiveFormExists) {
                console.log(`form does not exist at index ${index + 1}`);
                break;
              }

              await page.locator(repeatitiveFormLocator).dispatchEvent("click");
              await this.utils.resetForm(page);
              console.log(
                `form reseted for repeated form at index ${index + 1}`
              );
              index++;
              await page.goto(currentUrl);
              await page.waitForLoadState("domcontentloaded");
            }
            this.utils.formsReset.push(formName);

            if (index > 0) {
              console.log(`forms reset ${this.utils.formsReset}`);
              await page.goto(currentUrl);
              await page.waitForLoadState("domcontentloaded");
            }
          }
        }

        // await page.waitForLoadState("networkidle");
        const created = await this.createFormIfNotExists({
          eventGroupId,
          eventId,
          formId,
          formSequenceIndex,
        });
        if (created === true) {
          console.log("form created");
          await page.reload();
        }
        await page.waitForLoadState("domcontentloaded");
        await page.waitForTimeout(4000);

        const repeatitiveFormLocator = `(//div[contains(@class,'vdc_repeat_forms_page')]//table[contains(@class, 'vv_row_hover')]/tbody/tr[td/div])[${formSequenceIndex}]`;
        console.log(`repeatitiveFormLocator: ${repeatitiveFormLocator}`);
        const repeatitiveFormExists = await this.elementExists(
          page,
          repeatitiveFormLocator
        );

        if (!repeatitiveFormExists) {
          throw new Error(`${formName} form is not found`);
        }

        await page.locator(repeatitiveFormLocator).dispatchEvent("click");
      }
      console.log("after createFormIfNotExists");
      return { locatorExists: true, eventExisted: eventExisted };
    } catch (e: any) {
      console.error("Error:", e);
      return { locatorExists: false, eventExisted: true, error: e };
    }
  }

  async AssertEventOrForm({
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
    if (this.sessionId == null) {
      return false;
    }

    if (Action === "Event") {
      eventName = formName;
    }

    if (Action === "Form") {
      this.createEventIfNotExists(eventGroupName, eventName);
    }

    try {
      const response = await fetch(
        `https://${this.vaultDNS}/api/${this.version}/app/cdm/events?study_name=${this.studyName}&study_country=${this.studyCountry}&site=${this.siteName}&subject=${this.subjectName}`,
        {
          method: "GET",
          headers: {
            Authorization: `Bearer ${this.sessionId}`,
          },
        }
      );

      const data: any = await response.json();
      const events = data.events;

      let exists = false;

      events.forEach((event: any) => {
        if (
          Action === "Event" &&
          event.study_country === this.studyCountry &&
          event.site === this.siteName &&
          event.subject === this.subjectName &&
          event.event_name === eventName
        ) {
          exists = true;
          return;
        } else if (
          Action === "Form" &&
          event.study_country === this.studyCountry &&
          event.site === this.siteName &&
          event.subject === this.subjectName &&
          event.event_name === eventName
        ) {
          if (exists) {
            return;
          }
          const forms = event.forms;
          forms.forEach((form: any) => {
            if (form.form_name === formName) {
              exists = true;
              return;
            }
          });
        }
      });

      if (Expectation) {
        if (!exists) {
          if (Action === "Event") {
            throw new Error("Assertion failed: Event does not exist");
          } else if (Action === "Form") {
            throw new Error("Assertion failed: Form does not exist");
          }
        }
      } else {
        if (exists) {
          if (Action === "Event") {
            throw new Error("Assertion failed: Event exists");
          } else if (Action === "Form") {
            throw new Error("Assertion failed: Form exists");
          }
        }
      }
    } catch (e) {
      console.error("Error:", e);
      throw new Error(`Unable to assert due to ${e}`);
    }
  }

  async submitForm({
    eventGroupId,
    eventId,
    formId,
    formSequenceIndex = 1,
  }: {
    eventGroupId: string;
    eventId: string;
    formId: string;
    formSequenceIndex?: number;
  }) {
    if (this.sessionId == null) {
      return;
    }
    console.log("submit form body");
    console.log({
      study_name: this.studyName,
      forms: [
        {
          study_country: this.studyCountry,
          site: this.siteName,
          subject: this.subjectName,
          eventgroup_name: eventGroupId,
          event_name: eventId,
          form_name: formId,
          form_sequence: formSequenceIndex,
        },
      ],
    });
    try {
      const response = await fetch(
        `https://${this.vaultDNS}/api/${this.version}/app/cdm/forms/actions/submit`,
        {
          method: "POST",
          headers: {
            Accept: "application/json",
            "Content-Type": "application/json",
            Authorization: `Bearer ${this.sessionId}`,
          },
          body: JSON.stringify({
            study_name: this.studyName,
            forms: [
              {
                study_country: this.studyCountry,
                site: this.siteName,
                subject: this.subjectName,
                eventgroup_name: eventGroupId,
                event_name: eventId,
                form_name: formId,
                form_sequence: formSequenceIndex,
              },
            ],
          }),
        }
      );

      const responseData: any = await response.json();
      console.log("submit form response");
      console.log(responseData);
      if (
        responseData &&
        responseData.responseStatus.toLowerCase() === "success"
      ) {
        const forms = responseData.forms;
        let isFormSubmitted = false;
        forms.forEach((form: any) => {
          if (
            form.form_name === formId &&
            form.responseStatus.toLowerCase() === "success"
          ) {
            isFormSubmitted = true;
            console.log("Form submitted successfully");
          }
        });
        if (!isFormSubmitted) {
          throw new Error(`Form not submitted for ${formId}`);
        }
        return;
      }
    } catch (e) {
      console.error("Error:", e);
      console.log("Form submission failed");
      throw new Error(`Form submission failed for ${formId} due to ${e}`);
    }
    return;
  }

  async addItemGroup(
    itemGroupName: string,
    {
      eventGroupId,
      eventId,
      formId,
      formRepeatSequence = 1,
    }: {
      eventGroupId: string;
      eventId: string;
      formId: string;
      formRepeatSequence?: number;
    }
  ) {
    if (this.sessionId == null) {
      return;
    }

    try {
      const response = await fetch(
        `https://${this.vaultDNS}/api/${this.version}/app/cdm/forms?study_name=${this.studyName}&study_country=${this.studyCountry}&site=${this.siteName}&subject=${this.subjectName}&eventgroup_name=${eventGroupId}&event_name=${eventId}&form_name=${formId}`,
        {
          method: "GET",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${this.sessionId}`,
          },
        }
      );
      const responseData: any = await response.json();
      if (
        responseData &&
        responseData.responseStatus.toLowerCase() === "success"
      ) {
        const forms = responseData.forms;

        let isItemGroupPresent = false;

        forms.forEach((form: any) => {
          if (
            form.form_name === formId &&
            form.form_sequence === formRepeatSequence
          ) {
            const itemGroups = form.itemgroups;
            isItemGroupPresent = itemGroups.some(
              (itemGroup: any) => itemGroup.itemgroup_name === itemGroupName
            );
          }
        });
        if (!isItemGroupPresent) {
          console.log("item group is not present, creating one");
          try {
            const response = await fetch(
              `https://${this.vaultDNS}/api/${this.version}/app/cdm/itemgroups`,
              {
                method: "POST",
                headers: {
                  Accept: "application/json",
                  "Content-Type": "application/json",
                  Authorization: `Bearer ${this.sessionId}`,
                },
                body: JSON.stringify({
                  study_name: this.studyName,
                  itemgroups: [
                    {
                      study_country: this.studyCountry,
                      site: this.siteName,
                      subject: this.subjectName,
                      eventgroup_name: eventGroupId,
                      event_name: eventId,
                      form_name: formId,
                      itemgroup_name: itemGroupName,
                      form_sequence: formRepeatSequence,
                    },
                  ],
                }),
              }
            );

            const responseData: any = await response.json();
            if (
              responseData &&
              responseData.responseStatus.toLowerCase() === "success"
            ) {
              const itemGroups = responseData.itemgroups;
              let isItemGroupCreated = false;
              itemGroups.forEach((itemGroup: any) => {
                if (
                  itemGroup.itemgroup_name === itemGroupName &&
                  itemGroup.responseStatus.toLowerCase() === "success"
                ) {
                  console.log("item group created successfully");
                  isItemGroupCreated = true;
                  return true;
                }
              });
              return isItemGroupCreated;
            } else {
              console.log("Create Item Group API Failed");
              console.log(response);
              throw new Error(`Failed to create ${itemGroupName}`);
            }
          } catch (e) {
            console.log(`Failed to create item group ${e}`);
            throw new Error(
              `Failed to create ${itemGroupName} new section due to ${e}`
            );
          }
        } else {
          console.log("item group is already present");
          return false;
        }
      } else {
        console.log("Get Forms Response");
        console.log(responseData);
        throw new Error(
          `Failed to create ${itemGroupName} new section due to retrieve forms api failed`
        );
      }
    } catch (e) {
      console.error("Error:", e);
      console.log("item group creation failed");
      throw new Error(
        `Failed to create ${itemGroupName} new section due to ${e}`
      );
    }
    return;
  }

  async blurAllElements(page: Page, selector: string): Promise<void> {
    // Get all matching elements using the page.$$ method
    const elements = await page.$$(selector);

    if (elements.length === 0) {
      console.log(`No elements found for selector: ${selector}`);
      return;
    }

    console.log(
      `Found ${elements.length} elements matching selector: ${selector}`
    );

    // Dispatch blur event on each element
    for (const element of elements) {
      await element.evaluate((el) => el.dispatchEvent(new Event("blur")));
      console.log(`Blur event dispatched on element.`);
    }
  }

  public async retrieveForms({
    eventGroupId,
    eventId,
  }: {
    eventGroupId: string;
    eventId: string;
  }) {
    try {
      const response = await fetch(
        `https://${this.vaultDNS}/api/${this.version}/app/cdm/forms?study_name=${this.studyName}&study_country=${this.studyCountry}&site=${this.siteName}&subject=${this.subjectName}&eventgroup_name=${eventGroupId}&event_name=${eventId}`,
        {
          method: "GET",
          headers: {
            Accept: "application/json",
            "Content-Type": "application/json",
            Authorization: `Bearer ${this.sessionId}`,
          },
        }
      );

      const responseData: any = await response.json();
      if (
        responseData &&
        responseData.responseStatus.toLowerCase() === "success"
      ) {
        return responseData.forms;
      }
      console.log(responseData);
      throw new Error("Failed to retrieve forms");
    } catch (e) {
      console.error("Error:", e);
      console.log("form retrieval failed");
      throw new Error(`Failed to retrieve forms due to ${e}`);
    }
    return;
  }

  public async createFormIfNotExists({
    eventGroupId,
    eventId,
    formId,
    formSequenceIndex = 1,
  }: {
    eventGroupId: string;
    eventId: string;
    formId: string;
    formSequenceIndex?: number;
  }) {
    if (this.sessionId == null) {
      throw new Error(`Session is null to create repeated form ${formId}`);
    }

    try {
      const forms = await this.retrieveForms({ eventGroupId, eventId });
      if (forms && forms.length > 0) {
        let isFormPresent = false;
        let formsCount = 0;
        for (const form of forms) {
          if (form.form_name === formId) {
            formsCount++;
            if (formsCount >= formSequenceIndex) {
              isFormPresent = true;
              break;
            }
          }
        }

        if (!isFormPresent) {
          console.log("form is not present, creating one");
          await this.createForm({ eventGroupId, eventId, formId });
          return true;
        } else {
          console.log("form is already present");
        }
      } else {
        await this.createForm({ eventGroupId, eventId, formId });
        return true;
      }
    } catch (e) {
      console.error("Error:", e);
      console.log("Repeatitive Form Creation Failed");
      throw new Error(`Repeatitive Form Creation Failed for ${formId}`);
    }
    return;
  }

  public async createForm({
    eventGroupId,
    eventId,
    formId,
  }: {
    eventGroupId: string;
    eventId: string;
    formId: string;
  }) {
    if (this.sessionId == null) {
      return;
    }
    try {
      const response = await fetch(
        `https://${this.vaultDNS}/api/${this.version}/app/cdm/forms`,
        {
          method: "POST",
          headers: {
            Accept: "application/json",
            "Content-Type": "application/json",
            Authorization: `Bearer ${this.sessionId}`,
          },
          body: JSON.stringify({
            study_name: this.studyName,
            forms: [
              {
                study_country: this.studyCountry,
                site: this.siteName,
                subject: this.subjectName,
                eventgroup_name: eventGroupId,
                event_name: eventId,
                form_name: formId,
              },
            ],
          }),
        }
      );

      const responseData: any = await response.json();
      console.log(`form creation status ${responseData?.responseStatus}`);
      if (
        responseData &&
        responseData.responseStatus.toLowerCase() === "success"
      ) {
        return;
      }
      console.log("create form response");
      console.log(responseData);
      throw new Error(
        `Failed to create form for ${formId} with response ${responseData}`
      );
    } catch (e) {
      console.error("Error:", e);
      console.log("form creation failed");
      throw new Error(`Failed to create form for ${formId} due to ${e}`);
    }
  }

  public async ensureForms({
    eventGroupId,
    eventId,
    formId,
    count,
  }: {
    eventGroupId: string;
    eventId: string;
    formId: string;
    count: number;
  }) {
    if (this.sessionId == null) {
      return;
    }

    try {
      const forms = await this.retrieveForms({ eventGroupId, eventId });
      if (forms && forms.length > 0) {
      }
    } catch (e) {
      console.error("Error:", e);
      console.log("Repeatitive Form Creation Failed");
      throw new Error(`Repeatitive Form Creation Failed for ${formId}`);
    }
    return;
  }
}
