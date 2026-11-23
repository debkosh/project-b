// The glue between the form script (../forms.js) and DuckDuckGo's field matching
// (docs/password-autofill.md, "Detection"): their Matching and FormAnalyzer, used as they
// publish them, and called the way their own Scanner and Form call them. Their Scanner and
// Form themselves are left out: they belong to their apps, and draw into the page.
//
// tools/form-matching.sh bundles this file with their source into ../matching.gen.js. It
// gives the form script one function, which reads the page and changes nothing in it: no
// attribute is set, nothing is listened for, and nothing leaves but each field's kind.
import { createMatching } from 'ddg/src/Form/matching.js';
import FormAnalyzer from 'ddg/src/Form/FormAnalyzer.js';
import { isFormLikelyToBeUsedAsPageWrapper, isFormWithoutInteractiveControls } from 'ddg/src/autofill-utils.js';

// Their types that matter to a password manager, as the form script reports them.
const KINDS = {
  'credentials.username': 'username',
  'credentials.password': 'password',
  'credentials.password.current': 'password.current',
  'credentials.password.new': 'password.new',
  'credentials.totp': 'totp',
  'identities.emailAddress': 'email',
};

let matching;

// The element their Scanner takes for a field's form (Scanner.getParentForm): its form,
// unless that wraps most of the page, else the nearest thing around it that holds another
// field or a button.
function parentForm(input, known) {
  if (input.form && (known.has(input.form) || !isFormLikelyToBeUsedAsPageWrapper(input.form))) return input.form;
  let layers = 0;
  let element = input;
  while (layers <= 5 && element.parentElement && element.parentElement !== document.documentElement) {
    const sibling = [...element.parentElement.querySelectorAll('form')].find((f) => f !== element && !isFormWithoutInteractiveControls(f));
    if (sibling) return element;
    if (element instanceof HTMLFormElement) return element;
    element = element.parentElement;
    if (element.childElementCount > 1) {
      const inputs = element.querySelectorAll(matching.cssSelector('formInputsSelector'));
      const buttons = element.querySelectorAll(matching.cssSelector('submitButtonSelector'));
      if (inputs.length > 1 || buttons.length) return element;
      layers++;
    }
  }
  return input;
}

// For each of the frame's fields, in order: its kind, '' for none of KINDS, and the element
// their matching takes for its form.
globalThis.__pbFormMatching = (elements) => {
  matching ||= createMatching();
  const eligible = matching.cssSelector('formInputsSelector');
  const forms = new Map();
  const formOf = new Map();
  for (const input of elements) {
    // A field of one character is a box of a code split in pieces: their Form passes it over.
    if (!input.matches(eligible) || input.maxLength === 1) continue;
    const form = parentForm(input, forms);
    if (!forms.has(form)) forms.set(form, { analyzer: new FormAnalyzer(form, null, input, matching), inputs: [] });
    forms.get(form).inputs.push(input);
    formOf.set(input, form);
  }
  const types = new Map();
  for (const [form, f] of forms) {
    const opts = { isLogin: f.analyzer.isLogin, isHybrid: f.analyzer.isHybrid, isCCForm: f.analyzer.isCCForm(), hasCredentials: true, supportsIdentitiesAutofill: true };
    for (const input of f.inputs) types.set(input, matching.inferInputType(input, form, opts));
    // Their Form's rule (canCategorizeAmbiguousInput): in a sign-in with a password and no
    // username, the one field that is unknown, a phone or a card number is the username.
    const all = f.inputs.map((i) => types.get(i));
    if (f.analyzer.isLogin && !all.includes('credentials.username') && all.some((t) => t.startsWith('credentials.password'))) {
      const ambiguous = f.inputs.filter((i) => ['unknown', 'identities.phone', 'creditCards.cardNumber'].includes(types.get(i)));
      if (ambiguous.length === 1) types.set(ambiguous[0], 'credentials.username');
    }
  }
  return elements.map((el) => ({ kind: KINDS[types.get(el)] || '', form: formOf.get(el) || null }));
};
