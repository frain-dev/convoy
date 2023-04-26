/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import { isChrome, isEdge, isFirefox, isLinux, isMacintosh, isSafari, isWeb, isWindows } from '../../../base/common/platform.js';
import { isFalsyOrWhitespace } from '../../../base/common/strings.js';
import { createDecorator } from '../../instantiation/common/instantiation.js';
const CONSTANT_VALUES = new Map();
CONSTANT_VALUES.set('false', false);
CONSTANT_VALUES.set('true', true);
CONSTANT_VALUES.set('isMac', isMacintosh);
CONSTANT_VALUES.set('isLinux', isLinux);
CONSTANT_VALUES.set('isWindows', isWindows);
CONSTANT_VALUES.set('isWeb', isWeb);
CONSTANT_VALUES.set('isMacNative', isMacintosh && !isWeb);
CONSTANT_VALUES.set('isEdge', isEdge);
CONSTANT_VALUES.set('isFirefox', isFirefox);
CONSTANT_VALUES.set('isChrome', isChrome);
CONSTANT_VALUES.set('isSafari', isSafari);
const hasOwnProperty = Object.prototype.hasOwnProperty;
export class ContextKeyExpr {
    static has(key) {
        return ContextKeyDefinedExpr.create(key);
    }
    static equals(key, value) {
        return ContextKeyEqualsExpr.create(key, value);
    }
    static regex(key, value) {
        return ContextKeyRegexExpr.create(key, value);
    }
    static not(key) {
        return ContextKeyNotExpr.create(key);
    }
    static and(...expr) {
        return ContextKeyAndExpr.create(expr, null);
    }
    static or(...expr) {
        return ContextKeyOrExpr.create(expr, null, true);
    }
    static deserialize(serialized, strict = false) {
        if (!serialized) {
            return undefined;
        }
        return this._deserializeOrExpression(serialized, strict);
    }
    static _deserializeOrExpression(serialized, strict) {
        const pieces = serialized.split('||');
        return ContextKeyOrExpr.create(pieces.map(p => this._deserializeAndExpression(p, strict)), null, true);
    }
    static _deserializeAndExpression(serialized, strict) {
        const pieces = serialized.split('&&');
        return ContextKeyAndExpr.create(pieces.map(p => this._deserializeOne(p, strict)), null);
    }
    static _deserializeOne(serializedOne, strict) {
        serializedOne = serializedOne.trim();
        if (serializedOne.indexOf('!=') >= 0) {
            const pieces = serializedOne.split('!=');
            return ContextKeyNotEqualsExpr.create(pieces[0].trim(), this._deserializeValue(pieces[1], strict));
        }
        if (serializedOne.indexOf('==') >= 0) {
            const pieces = serializedOne.split('==');
            return ContextKeyEqualsExpr.create(pieces[0].trim(), this._deserializeValue(pieces[1], strict));
        }
        if (serializedOne.indexOf('=~') >= 0) {
            const pieces = serializedOne.split('=~');
            return ContextKeyRegexExpr.create(pieces[0].trim(), this._deserializeRegexValue(pieces[1], strict));
        }
        if (serializedOne.indexOf(' not in ') >= 0) {
            const pieces = serializedOne.split(' not in ');
            return ContextKeyNotInExpr.create(pieces[0].trim(), pieces[1].trim());
        }
        if (serializedOne.indexOf(' in ') >= 0) {
            const pieces = serializedOne.split(' in ');
            return ContextKeyInExpr.create(pieces[0].trim(), pieces[1].trim());
        }
        if (/^[^<=>]+>=[^<=>]+$/.test(serializedOne)) {
            const pieces = serializedOne.split('>=');
            return ContextKeyGreaterEqualsExpr.create(pieces[0].trim(), pieces[1].trim());
        }
        if (/^[^<=>]+>[^<=>]+$/.test(serializedOne)) {
            const pieces = serializedOne.split('>');
            return ContextKeyGreaterExpr.create(pieces[0].trim(), pieces[1].trim());
        }
        if (/^[^<=>]+<=[^<=>]+$/.test(serializedOne)) {
            const pieces = serializedOne.split('<=');
            return ContextKeySmallerEqualsExpr.create(pieces[0].trim(), pieces[1].trim());
        }
        if (/^[^<=>]+<[^<=>]+$/.test(serializedOne)) {
            const pieces = serializedOne.split('<');
            return ContextKeySmallerExpr.create(pieces[0].trim(), pieces[1].trim());
        }
        if (/^\!\s*/.test(serializedOne)) {
            return ContextKeyNotExpr.create(serializedOne.substr(1).trim());
        }
        return ContextKeyDefinedExpr.create(serializedOne);
    }
    static _deserializeValue(serializedValue, strict) {
        serializedValue = serializedValue.trim();
        if (serializedValue === 'true') {
            return true;
        }
        if (serializedValue === 'false') {
            return false;
        }
        const m = /^'([^']*)'$/.exec(serializedValue);
        if (m) {
            return m[1].trim();
        }
        return serializedValue;
    }
    static _deserializeRegexValue(serializedValue, strict) {
        if (isFalsyOrWhitespace(serializedValue)) {
            if (strict) {
                throw new Error('missing regexp-value for =~-expression');
            }
            else {
                console.warn('missing regexp-value for =~-expression');
            }
            return null;
        }
        const start = serializedValue.indexOf('/');
        const end = serializedValue.lastIndexOf('/');
        if (start === end || start < 0 /* || to < 0 */) {
            if (strict) {
                throw new Error(`bad regexp-value '${serializedValue}', missing /-enclosure`);
            }
            else {
                console.warn(`bad regexp-value '${serializedValue}', missing /-enclosure`);
            }
            return null;
        }
        const value = serializedValue.slice(start + 1, end);
        const caseIgnoreFlag = serializedValue[end + 1] === 'i' ? 'i' : '';
        try {
            return new RegExp(value, caseIgnoreFlag);
        }
        catch (e) {
            if (strict) {
                throw new Error(`bad regexp-value '${serializedValue}', parse error: ${e}`);
            }
            else {
                console.warn(`bad regexp-value '${serializedValue}', parse error: ${e}`);
            }
            return null;
        }
    }
}
export function expressionsAreEqualWithConstantSubstitution(a, b) {
    const aExpr = a ? a.substituteConstants() : undefined;
    const bExpr = b ? b.substituteConstants() : undefined;
    if (!aExpr && !bExpr) {
        return true;
    }
    if (!aExpr || !bExpr) {
        return false;
    }
    return aExpr.equals(bExpr);
}
function cmp(a, b) {
    return a.cmp(b);
}
export class ContextKeyFalseExpr {
    constructor() {
        this.type = 0 /* ContextKeyExprType.False */;
    }
    cmp(other) {
        return this.type - other.type;
    }
    equals(other) {
        return (other.type === this.type);
    }
    substituteConstants() {
        return this;
    }
    evaluate(context) {
        return false;
    }
    serialize() {
        return 'false';
    }
    keys() {
        return [];
    }
    negate() {
        return ContextKeyTrueExpr.INSTANCE;
    }
}
ContextKeyFalseExpr.INSTANCE = new ContextKeyFalseExpr();
export class ContextKeyTrueExpr {
    constructor() {
        this.type = 1 /* ContextKeyExprType.True */;
    }
    cmp(other) {
        return this.type - other.type;
    }
    equals(other) {
        return (other.type === this.type);
    }
    substituteConstants() {
        return this;
    }
    evaluate(context) {
        return true;
    }
    serialize() {
        return 'true';
    }
    keys() {
        return [];
    }
    negate() {
        return ContextKeyFalseExpr.INSTANCE;
    }
}
ContextKeyTrueExpr.INSTANCE = new ContextKeyTrueExpr();
export class ContextKeyDefinedExpr {
    constructor(key, negated) {
        this.key = key;
        this.negated = negated;
        this.type = 2 /* ContextKeyExprType.Defined */;
    }
    static create(key, negated = null) {
        const constantValue = CONSTANT_VALUES.get(key);
        if (typeof constantValue === 'boolean') {
            return constantValue ? ContextKeyTrueExpr.INSTANCE : ContextKeyFalseExpr.INSTANCE;
        }
        return new ContextKeyDefinedExpr(key, negated);
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        return cmp1(this.key, other.key);
    }
    equals(other) {
        if (other.type === this.type) {
            return (this.key === other.key);
        }
        return false;
    }
    substituteConstants() {
        const constantValue = CONSTANT_VALUES.get(this.key);
        if (typeof constantValue === 'boolean') {
            return constantValue ? ContextKeyTrueExpr.INSTANCE : ContextKeyFalseExpr.INSTANCE;
        }
        return this;
    }
    evaluate(context) {
        return (!!context.getValue(this.key));
    }
    serialize() {
        return this.key;
    }
    keys() {
        return [this.key];
    }
    negate() {
        if (!this.negated) {
            this.negated = ContextKeyNotExpr.create(this.key, this);
        }
        return this.negated;
    }
}
export class ContextKeyEqualsExpr {
    constructor(key, value, negated) {
        this.key = key;
        this.value = value;
        this.negated = negated;
        this.type = 4 /* ContextKeyExprType.Equals */;
    }
    static create(key, value, negated = null) {
        if (typeof value === 'boolean') {
            return (value ? ContextKeyDefinedExpr.create(key, negated) : ContextKeyNotExpr.create(key, negated));
        }
        const constantValue = CONSTANT_VALUES.get(key);
        if (typeof constantValue === 'boolean') {
            const trueValue = constantValue ? 'true' : 'false';
            return (value === trueValue ? ContextKeyTrueExpr.INSTANCE : ContextKeyFalseExpr.INSTANCE);
        }
        return new ContextKeyEqualsExpr(key, value, negated);
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        return cmp2(this.key, this.value, other.key, other.value);
    }
    equals(other) {
        if (other.type === this.type) {
            return (this.key === other.key && this.value === other.value);
        }
        return false;
    }
    substituteConstants() {
        const constantValue = CONSTANT_VALUES.get(this.key);
        if (typeof constantValue === 'boolean') {
            const trueValue = constantValue ? 'true' : 'false';
            return (this.value === trueValue ? ContextKeyTrueExpr.INSTANCE : ContextKeyFalseExpr.INSTANCE);
        }
        return this;
    }
    evaluate(context) {
        // Intentional ==
        // eslint-disable-next-line eqeqeq
        return (context.getValue(this.key) == this.value);
    }
    serialize() {
        return `${this.key} == '${this.value}'`;
    }
    keys() {
        return [this.key];
    }
    negate() {
        if (!this.negated) {
            this.negated = ContextKeyNotEqualsExpr.create(this.key, this.value, this);
        }
        return this.negated;
    }
}
export class ContextKeyInExpr {
    constructor(key, valueKey) {
        this.key = key;
        this.valueKey = valueKey;
        this.type = 10 /* ContextKeyExprType.In */;
        this.negated = null;
    }
    static create(key, valueKey) {
        return new ContextKeyInExpr(key, valueKey);
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        return cmp2(this.key, this.valueKey, other.key, other.valueKey);
    }
    equals(other) {
        if (other.type === this.type) {
            return (this.key === other.key && this.valueKey === other.valueKey);
        }
        return false;
    }
    substituteConstants() {
        return this;
    }
    evaluate(context) {
        const source = context.getValue(this.valueKey);
        const item = context.getValue(this.key);
        if (Array.isArray(source)) {
            return source.includes(item);
        }
        if (typeof item === 'string' && typeof source === 'object' && source !== null) {
            return hasOwnProperty.call(source, item);
        }
        return false;
    }
    serialize() {
        return `${this.key} in '${this.valueKey}'`;
    }
    keys() {
        return [this.key, this.valueKey];
    }
    negate() {
        if (!this.negated) {
            this.negated = ContextKeyNotInExpr.create(this.key, this.valueKey);
        }
        return this.negated;
    }
}
export class ContextKeyNotInExpr {
    constructor(key, valueKey) {
        this.key = key;
        this.valueKey = valueKey;
        this.type = 11 /* ContextKeyExprType.NotIn */;
        this._negated = ContextKeyInExpr.create(key, valueKey);
    }
    static create(key, valueKey) {
        return new ContextKeyNotInExpr(key, valueKey);
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        return this._negated.cmp(other._negated);
    }
    equals(other) {
        if (other.type === this.type) {
            return this._negated.equals(other._negated);
        }
        return false;
    }
    substituteConstants() {
        return this;
    }
    evaluate(context) {
        return !this._negated.evaluate(context);
    }
    serialize() {
        return `${this.key} not in '${this.valueKey}'`;
    }
    keys() {
        return this._negated.keys();
    }
    negate() {
        return this._negated;
    }
}
export class ContextKeyNotEqualsExpr {
    constructor(key, value, negated) {
        this.key = key;
        this.value = value;
        this.negated = negated;
        this.type = 5 /* ContextKeyExprType.NotEquals */;
    }
    static create(key, value, negated = null) {
        if (typeof value === 'boolean') {
            if (value) {
                return ContextKeyNotExpr.create(key, negated);
            }
            return ContextKeyDefinedExpr.create(key, negated);
        }
        const constantValue = CONSTANT_VALUES.get(key);
        if (typeof constantValue === 'boolean') {
            const falseValue = constantValue ? 'true' : 'false';
            return (value === falseValue ? ContextKeyFalseExpr.INSTANCE : ContextKeyTrueExpr.INSTANCE);
        }
        return new ContextKeyNotEqualsExpr(key, value, negated);
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        return cmp2(this.key, this.value, other.key, other.value);
    }
    equals(other) {
        if (other.type === this.type) {
            return (this.key === other.key && this.value === other.value);
        }
        return false;
    }
    substituteConstants() {
        const constantValue = CONSTANT_VALUES.get(this.key);
        if (typeof constantValue === 'boolean') {
            const falseValue = constantValue ? 'true' : 'false';
            return (this.value === falseValue ? ContextKeyFalseExpr.INSTANCE : ContextKeyTrueExpr.INSTANCE);
        }
        return this;
    }
    evaluate(context) {
        // Intentional !=
        // eslint-disable-next-line eqeqeq
        return (context.getValue(this.key) != this.value);
    }
    serialize() {
        return `${this.key} != '${this.value}'`;
    }
    keys() {
        return [this.key];
    }
    negate() {
        if (!this.negated) {
            this.negated = ContextKeyEqualsExpr.create(this.key, this.value, this);
        }
        return this.negated;
    }
}
export class ContextKeyNotExpr {
    constructor(key, negated) {
        this.key = key;
        this.negated = negated;
        this.type = 3 /* ContextKeyExprType.Not */;
    }
    static create(key, negated = null) {
        const constantValue = CONSTANT_VALUES.get(key);
        if (typeof constantValue === 'boolean') {
            return (constantValue ? ContextKeyFalseExpr.INSTANCE : ContextKeyTrueExpr.INSTANCE);
        }
        return new ContextKeyNotExpr(key, negated);
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        return cmp1(this.key, other.key);
    }
    equals(other) {
        if (other.type === this.type) {
            return (this.key === other.key);
        }
        return false;
    }
    substituteConstants() {
        const constantValue = CONSTANT_VALUES.get(this.key);
        if (typeof constantValue === 'boolean') {
            return (constantValue ? ContextKeyFalseExpr.INSTANCE : ContextKeyTrueExpr.INSTANCE);
        }
        return this;
    }
    evaluate(context) {
        return (!context.getValue(this.key));
    }
    serialize() {
        return `!${this.key}`;
    }
    keys() {
        return [this.key];
    }
    negate() {
        if (!this.negated) {
            this.negated = ContextKeyDefinedExpr.create(this.key, this);
        }
        return this.negated;
    }
}
function withFloatOrStr(value, callback) {
    if (typeof value === 'string') {
        const n = parseFloat(value);
        if (!isNaN(n)) {
            value = n;
        }
    }
    if (typeof value === 'string' || typeof value === 'number') {
        return callback(value);
    }
    return ContextKeyFalseExpr.INSTANCE;
}
export class ContextKeyGreaterExpr {
    constructor(key, value, negated) {
        this.key = key;
        this.value = value;
        this.negated = negated;
        this.type = 12 /* ContextKeyExprType.Greater */;
    }
    static create(key, _value, negated = null) {
        return withFloatOrStr(_value, (value) => new ContextKeyGreaterExpr(key, value, negated));
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        return cmp2(this.key, this.value, other.key, other.value);
    }
    equals(other) {
        if (other.type === this.type) {
            return (this.key === other.key && this.value === other.value);
        }
        return false;
    }
    substituteConstants() {
        return this;
    }
    evaluate(context) {
        if (typeof this.value === 'string') {
            return false;
        }
        return (parseFloat(context.getValue(this.key)) > this.value);
    }
    serialize() {
        return `${this.key} > ${this.value}`;
    }
    keys() {
        return [this.key];
    }
    negate() {
        if (!this.negated) {
            this.negated = ContextKeySmallerEqualsExpr.create(this.key, this.value, this);
        }
        return this.negated;
    }
}
export class ContextKeyGreaterEqualsExpr {
    constructor(key, value, negated) {
        this.key = key;
        this.value = value;
        this.negated = negated;
        this.type = 13 /* ContextKeyExprType.GreaterEquals */;
    }
    static create(key, _value, negated = null) {
        return withFloatOrStr(_value, (value) => new ContextKeyGreaterEqualsExpr(key, value, negated));
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        return cmp2(this.key, this.value, other.key, other.value);
    }
    equals(other) {
        if (other.type === this.type) {
            return (this.key === other.key && this.value === other.value);
        }
        return false;
    }
    substituteConstants() {
        return this;
    }
    evaluate(context) {
        if (typeof this.value === 'string') {
            return false;
        }
        return (parseFloat(context.getValue(this.key)) >= this.value);
    }
    serialize() {
        return `${this.key} >= ${this.value}`;
    }
    keys() {
        return [this.key];
    }
    negate() {
        if (!this.negated) {
            this.negated = ContextKeySmallerExpr.create(this.key, this.value, this);
        }
        return this.negated;
    }
}
export class ContextKeySmallerExpr {
    constructor(key, value, negated) {
        this.key = key;
        this.value = value;
        this.negated = negated;
        this.type = 14 /* ContextKeyExprType.Smaller */;
    }
    static create(key, _value, negated = null) {
        return withFloatOrStr(_value, (value) => new ContextKeySmallerExpr(key, value, negated));
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        return cmp2(this.key, this.value, other.key, other.value);
    }
    equals(other) {
        if (other.type === this.type) {
            return (this.key === other.key && this.value === other.value);
        }
        return false;
    }
    substituteConstants() {
        return this;
    }
    evaluate(context) {
        if (typeof this.value === 'string') {
            return false;
        }
        return (parseFloat(context.getValue(this.key)) < this.value);
    }
    serialize() {
        return `${this.key} < ${this.value}`;
    }
    keys() {
        return [this.key];
    }
    negate() {
        if (!this.negated) {
            this.negated = ContextKeyGreaterEqualsExpr.create(this.key, this.value, this);
        }
        return this.negated;
    }
}
export class ContextKeySmallerEqualsExpr {
    constructor(key, value, negated) {
        this.key = key;
        this.value = value;
        this.negated = negated;
        this.type = 15 /* ContextKeyExprType.SmallerEquals */;
    }
    static create(key, _value, negated = null) {
        return withFloatOrStr(_value, (value) => new ContextKeySmallerEqualsExpr(key, value, negated));
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        return cmp2(this.key, this.value, other.key, other.value);
    }
    equals(other) {
        if (other.type === this.type) {
            return (this.key === other.key && this.value === other.value);
        }
        return false;
    }
    substituteConstants() {
        return this;
    }
    evaluate(context) {
        if (typeof this.value === 'string') {
            return false;
        }
        return (parseFloat(context.getValue(this.key)) <= this.value);
    }
    serialize() {
        return `${this.key} <= ${this.value}`;
    }
    keys() {
        return [this.key];
    }
    negate() {
        if (!this.negated) {
            this.negated = ContextKeyGreaterExpr.create(this.key, this.value, this);
        }
        return this.negated;
    }
}
export class ContextKeyRegexExpr {
    constructor(key, regexp) {
        this.key = key;
        this.regexp = regexp;
        this.type = 7 /* ContextKeyExprType.Regex */;
        this.negated = null;
        //
    }
    static create(key, regexp) {
        return new ContextKeyRegexExpr(key, regexp);
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        if (this.key < other.key) {
            return -1;
        }
        if (this.key > other.key) {
            return 1;
        }
        const thisSource = this.regexp ? this.regexp.source : '';
        const otherSource = other.regexp ? other.regexp.source : '';
        if (thisSource < otherSource) {
            return -1;
        }
        if (thisSource > otherSource) {
            return 1;
        }
        return 0;
    }
    equals(other) {
        if (other.type === this.type) {
            const thisSource = this.regexp ? this.regexp.source : '';
            const otherSource = other.regexp ? other.regexp.source : '';
            return (this.key === other.key && thisSource === otherSource);
        }
        return false;
    }
    substituteConstants() {
        return this;
    }
    evaluate(context) {
        const value = context.getValue(this.key);
        return this.regexp ? this.regexp.test(value) : false;
    }
    serialize() {
        const value = this.regexp
            ? `/${this.regexp.source}/${this.regexp.ignoreCase ? 'i' : ''}`
            : '/invalid/';
        return `${this.key} =~ ${value}`;
    }
    keys() {
        return [this.key];
    }
    negate() {
        if (!this.negated) {
            this.negated = ContextKeyNotRegexExpr.create(this);
        }
        return this.negated;
    }
}
export class ContextKeyNotRegexExpr {
    constructor(_actual) {
        this._actual = _actual;
        this.type = 8 /* ContextKeyExprType.NotRegex */;
        //
    }
    static create(actual) {
        return new ContextKeyNotRegexExpr(actual);
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        return this._actual.cmp(other._actual);
    }
    equals(other) {
        if (other.type === this.type) {
            return this._actual.equals(other._actual);
        }
        return false;
    }
    substituteConstants() {
        return this;
    }
    evaluate(context) {
        return !this._actual.evaluate(context);
    }
    serialize() {
        throw new Error('Method not implemented.');
    }
    keys() {
        return this._actual.keys();
    }
    negate() {
        return this._actual;
    }
}
/**
 * @returns the same instance if nothing changed.
 */
function eliminateConstantsInArray(arr) {
    // Allocate array only if there is a difference
    let newArr = null;
    for (let i = 0, len = arr.length; i < len; i++) {
        const newExpr = arr[i].substituteConstants();
        if (arr[i] !== newExpr) {
            // something has changed!
            // allocate array on first difference
            if (newArr === null) {
                newArr = [];
                for (let j = 0; j < i; j++) {
                    newArr[j] = arr[j];
                }
            }
        }
        if (newArr !== null) {
            newArr[i] = newExpr;
        }
    }
    if (newArr === null) {
        return arr;
    }
    return newArr;
}
class ContextKeyAndExpr {
    constructor(expr, negated) {
        this.expr = expr;
        this.negated = negated;
        this.type = 6 /* ContextKeyExprType.And */;
    }
    static create(_expr, negated) {
        return ContextKeyAndExpr._normalizeArr(_expr, negated);
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        if (this.expr.length < other.expr.length) {
            return -1;
        }
        if (this.expr.length > other.expr.length) {
            return 1;
        }
        for (let i = 0, len = this.expr.length; i < len; i++) {
            const r = cmp(this.expr[i], other.expr[i]);
            if (r !== 0) {
                return r;
            }
        }
        return 0;
    }
    equals(other) {
        if (other.type === this.type) {
            if (this.expr.length !== other.expr.length) {
                return false;
            }
            for (let i = 0, len = this.expr.length; i < len; i++) {
                if (!this.expr[i].equals(other.expr[i])) {
                    return false;
                }
            }
            return true;
        }
        return false;
    }
    substituteConstants() {
        const exprArr = eliminateConstantsInArray(this.expr);
        if (exprArr === this.expr) {
            // no change
            return this;
        }
        return ContextKeyAndExpr.create(exprArr, this.negated);
    }
    evaluate(context) {
        for (let i = 0, len = this.expr.length; i < len; i++) {
            if (!this.expr[i].evaluate(context)) {
                return false;
            }
        }
        return true;
    }
    static _normalizeArr(arr, negated) {
        const expr = [];
        let hasTrue = false;
        for (const e of arr) {
            if (!e) {
                continue;
            }
            if (e.type === 1 /* ContextKeyExprType.True */) {
                // anything && true ==> anything
                hasTrue = true;
                continue;
            }
            if (e.type === 0 /* ContextKeyExprType.False */) {
                // anything && false ==> false
                return ContextKeyFalseExpr.INSTANCE;
            }
            if (e.type === 6 /* ContextKeyExprType.And */) {
                expr.push(...e.expr);
                continue;
            }
            expr.push(e);
        }
        if (expr.length === 0 && hasTrue) {
            return ContextKeyTrueExpr.INSTANCE;
        }
        if (expr.length === 0) {
            return undefined;
        }
        if (expr.length === 1) {
            return expr[0];
        }
        expr.sort(cmp);
        // eliminate duplicate terms
        for (let i = 1; i < expr.length; i++) {
            if (expr[i - 1].equals(expr[i])) {
                expr.splice(i, 1);
                i--;
            }
        }
        if (expr.length === 1) {
            return expr[0];
        }
        // We must distribute any OR expression because we don't support parens
        // OR extensions will be at the end (due to sorting rules)
        while (expr.length > 1) {
            const lastElement = expr[expr.length - 1];
            if (lastElement.type !== 9 /* ContextKeyExprType.Or */) {
                break;
            }
            // pop the last element
            expr.pop();
            // pop the second to last element
            const secondToLastElement = expr.pop();
            const isFinished = (expr.length === 0);
            // distribute `lastElement` over `secondToLastElement`
            const resultElement = ContextKeyOrExpr.create(lastElement.expr.map(el => ContextKeyAndExpr.create([el, secondToLastElement], null)), null, isFinished);
            if (resultElement) {
                expr.push(resultElement);
                expr.sort(cmp);
            }
        }
        if (expr.length === 1) {
            return expr[0];
        }
        return new ContextKeyAndExpr(expr, negated);
    }
    serialize() {
        return this.expr.map(e => e.serialize()).join(' && ');
    }
    keys() {
        const result = [];
        for (const expr of this.expr) {
            result.push(...expr.keys());
        }
        return result;
    }
    negate() {
        if (!this.negated) {
            const result = [];
            for (const expr of this.expr) {
                result.push(expr.negate());
            }
            this.negated = ContextKeyOrExpr.create(result, this, true);
        }
        return this.negated;
    }
}
class ContextKeyOrExpr {
    constructor(expr, negated) {
        this.expr = expr;
        this.negated = negated;
        this.type = 9 /* ContextKeyExprType.Or */;
    }
    static create(_expr, negated, extraRedundantCheck) {
        return ContextKeyOrExpr._normalizeArr(_expr, negated, extraRedundantCheck);
    }
    cmp(other) {
        if (other.type !== this.type) {
            return this.type - other.type;
        }
        if (this.expr.length < other.expr.length) {
            return -1;
        }
        if (this.expr.length > other.expr.length) {
            return 1;
        }
        for (let i = 0, len = this.expr.length; i < len; i++) {
            const r = cmp(this.expr[i], other.expr[i]);
            if (r !== 0) {
                return r;
            }
        }
        return 0;
    }
    equals(other) {
        if (other.type === this.type) {
            if (this.expr.length !== other.expr.length) {
                return false;
            }
            for (let i = 0, len = this.expr.length; i < len; i++) {
                if (!this.expr[i].equals(other.expr[i])) {
                    return false;
                }
            }
            return true;
        }
        return false;
    }
    substituteConstants() {
        const exprArr = eliminateConstantsInArray(this.expr);
        if (exprArr === this.expr) {
            // no change
            return this;
        }
        return ContextKeyOrExpr.create(exprArr, this.negated, false);
    }
    evaluate(context) {
        for (let i = 0, len = this.expr.length; i < len; i++) {
            if (this.expr[i].evaluate(context)) {
                return true;
            }
        }
        return false;
    }
    static _normalizeArr(arr, negated, extraRedundantCheck) {
        let expr = [];
        let hasFalse = false;
        if (arr) {
            for (let i = 0, len = arr.length; i < len; i++) {
                const e = arr[i];
                if (!e) {
                    continue;
                }
                if (e.type === 0 /* ContextKeyExprType.False */) {
                    // anything || false ==> anything
                    hasFalse = true;
                    continue;
                }
                if (e.type === 1 /* ContextKeyExprType.True */) {
                    // anything || true ==> true
                    return ContextKeyTrueExpr.INSTANCE;
                }
                if (e.type === 9 /* ContextKeyExprType.Or */) {
                    expr = expr.concat(e.expr);
                    continue;
                }
                expr.push(e);
            }
            if (expr.length === 0 && hasFalse) {
                return ContextKeyFalseExpr.INSTANCE;
            }
            expr.sort(cmp);
        }
        if (expr.length === 0) {
            return undefined;
        }
        if (expr.length === 1) {
            return expr[0];
        }
        // eliminate duplicate terms
        for (let i = 1; i < expr.length; i++) {
            if (expr[i - 1].equals(expr[i])) {
                expr.splice(i, 1);
                i--;
            }
        }
        if (expr.length === 1) {
            return expr[0];
        }
        // eliminate redundant terms
        if (extraRedundantCheck) {
            for (let i = 0; i < expr.length; i++) {
                for (let j = i + 1; j < expr.length; j++) {
                    if (implies(expr[i], expr[j])) {
                        expr.splice(j, 1);
                        j--;
                    }
                }
            }
            if (expr.length === 1) {
                return expr[0];
            }
        }
        return new ContextKeyOrExpr(expr, negated);
    }
    serialize() {
        return this.expr.map(e => e.serialize()).join(' || ');
    }
    keys() {
        const result = [];
        for (const expr of this.expr) {
            result.push(...expr.keys());
        }
        return result;
    }
    negate() {
        if (!this.negated) {
            const result = [];
            for (const expr of this.expr) {
                result.push(expr.negate());
            }
            // We don't support parens, so here we distribute the AND over the OR terminals
            // We always take the first 2 AND pairs and distribute them
            while (result.length > 1) {
                const LEFT = result.shift();
                const RIGHT = result.shift();
                const all = [];
                for (const left of getTerminals(LEFT)) {
                    for (const right of getTerminals(RIGHT)) {
                        all.push(ContextKeyAndExpr.create([left, right], null));
                    }
                }
                const isFinished = (result.length === 0);
                result.unshift(ContextKeyOrExpr.create(all, null, isFinished));
            }
            this.negated = result[0];
        }
        return this.negated;
    }
}
export class RawContextKey extends ContextKeyDefinedExpr {
    constructor(key, defaultValue, metaOrHide) {
        super(key, null);
        this._defaultValue = defaultValue;
        // collect all context keys into a central place
        if (typeof metaOrHide === 'object') {
            RawContextKey._info.push(Object.assign(Object.assign({}, metaOrHide), { key }));
        }
        else if (metaOrHide !== true) {
            RawContextKey._info.push({ key, description: metaOrHide, type: defaultValue !== null && defaultValue !== undefined ? typeof defaultValue : undefined });
        }
    }
    static all() {
        return RawContextKey._info.values();
    }
    bindTo(target) {
        return target.createKey(this.key, this._defaultValue);
    }
    getValue(target) {
        return target.getContextKeyValue(this.key);
    }
    toNegated() {
        return this.negate();
    }
    isEqualTo(value) {
        return ContextKeyEqualsExpr.create(this.key, value);
    }
}
RawContextKey._info = [];
export const IContextKeyService = createDecorator('contextKeyService');
export const SET_CONTEXT_COMMAND_ID = 'setContext';
function cmp1(key1, key2) {
    if (key1 < key2) {
        return -1;
    }
    if (key1 > key2) {
        return 1;
    }
    return 0;
}
function cmp2(key1, value1, key2, value2) {
    if (key1 < key2) {
        return -1;
    }
    if (key1 > key2) {
        return 1;
    }
    if (value1 < value2) {
        return -1;
    }
    if (value1 > value2) {
        return 1;
    }
    return 0;
}
/**
 * Returns true if it is provable `p` implies `q`.
 */
export function implies(p, q) {
    if (q.type === 6 /* ContextKeyExprType.And */ && (p.type !== 9 /* ContextKeyExprType.Or */ && p.type !== 6 /* ContextKeyExprType.And */)) {
        // covers the case: A implies A && B
        for (const qTerm of q.expr) {
            if (p.equals(qTerm)) {
                return true;
            }
        }
    }
    const notP = p.negate();
    const expr = getTerminals(notP).concat(getTerminals(q));
    expr.sort(cmp);
    for (let i = 0; i < expr.length; i++) {
        const a = expr[i];
        const notA = a.negate();
        for (let j = i + 1; j < expr.length; j++) {
            const b = expr[j];
            if (notA.equals(b)) {
                return true;
            }
        }
    }
    return false;
}
function getTerminals(node) {
    if (node.type === 9 /* ContextKeyExprType.Or */) {
        return node.expr;
    }
    return [node];
}
