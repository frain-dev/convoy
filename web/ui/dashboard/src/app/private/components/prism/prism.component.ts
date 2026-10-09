import {AfterViewInit, Component, ElementRef, Input, OnChanges, ViewChild} from '@angular/core';
import * as Prism from 'prismjs';
import 'prismjs/components/prism-javascript';
import 'prismjs/components/prism-scss';
import 'prismjs/components/prism-json';
import 'prismjs/plugins/line-numbers/prism-line-numbers';

@Component({
    selector: 'prism',
    templateUrl: './prism.component.html',
    styleUrls: ['./prism.component.scss'],
    standalone: false
})
export class PrismComponent implements AfterViewInit, OnChanges {
	@ViewChild('codeEle') codeEle!: ElementRef;
	@Input() code?: string;
	@Input() language?: string;
	@Input('title') title?: string;
	@Input('type') type?: 'default' | 'headers' | 'display' = 'default';
	@Input() showPayload = false;
	@Input() highlightTerms: string[] = [];
	@Input() highlightKeyTerms: string[] = [];
	@Input() highlightValueTerms: string[] = [];
	@Input() autoScrollKey = '';

	private lastAutoScrollKey = '';

	constructor() {}

	ngAfterViewInit() {
		if (this.type !== 'headers') {
			Prism.highlightElement(this.codeEle?.nativeElement);
			this.applySearchHighlights();
		}
	}

	ngOnChanges(changes: import('@angular/core').SimpleChanges): void {
		if (changes['autoScrollKey'] && !this.autoScrollKey) {
			this.lastAutoScrollKey = '';
		}

		const shouldRefresh =
			!!changes['code'] ||
			!!changes['highlightTerms'] ||
			!!changes['highlightKeyTerms'] ||
			!!changes['highlightValueTerms'];

		if (shouldRefresh && this.codeEle?.nativeElement && this.type !== 'headers') {
			this.codeEle.nativeElement.textContent = this.code;
			Prism.highlightElement(this.codeEle.nativeElement);
			this.applySearchHighlights();
		}
	}

	private applySearchHighlights(): void {
		if (this.type === 'headers' || !this.codeEle?.nativeElement) {
			return;
		}

		const valueTerms = this.mergeHighlightTerms(this.highlightValueTerms, this.highlightTerms);
		const keyTerms = this.mergeHighlightTerms(this.highlightKeyTerms);
		if (!valueTerms.length && !keyTerms.length) {
			return;
		}

		const valueMarks = this.wrapValueTermsInTextNodes(this.codeEle.nativeElement, valueTerms);
		if (keyTerms.length && valueMarks.length) {
			this.highlightMatchingPropertyKeys(valueMarks, keyTerms);
		}
		this.maybeScrollToFirstHighlight();
	}

	private mergeHighlightTerms(...groups: string[][]): string[] {
		return [...new Set(groups.flatMap(group => group.map(term => term?.trim()).filter(Boolean)))].sort((a, b) => b.length - a.length);
	}

	private maybeScrollToFirstHighlight(): void {
		if (!this.autoScrollKey || this.autoScrollKey === this.lastAutoScrollKey) {
			return;
		}
		if (!this.mergeHighlightTerms(this.highlightValueTerms, this.highlightTerms, this.highlightKeyTerms).length) {
			return;
		}

		const scrollKey = this.autoScrollKey;
		this.lastAutoScrollKey = scrollKey;

		const scroll = () => {
			if (this.autoScrollKey !== scrollKey) {
				return;
			}

			const mark = this.codeEle?.nativeElement?.querySelector('mark.search-hit') as HTMLElement | null;
			if (!mark) {
				return;
			}

			const scrollRoot = this.codeEle?.nativeElement?.closest('pre') as HTMLElement | null;
			if (!scrollRoot) {
				return;
			}

			this.scrollMarkWithinContainer(mark, scrollRoot);
		};

		if (this.code && this.code.length > 300 && !this.showPayload) {
			this.showPayload = true;
			setTimeout(scroll, 50);
			return;
		}

		setTimeout(scroll, 0);
	}

	private scrollMarkWithinContainer(mark: HTMLElement, scrollRoot: HTMLElement): void {
		const padding = 12;
		const rootRect = scrollRoot.getBoundingClientRect();
		const markRect = mark.getBoundingClientRect();
		const markTop = markRect.top - rootRect.top + scrollRoot.scrollTop;
		const markBottom = markTop + markRect.height;
		const viewTop = scrollRoot.scrollTop;
		const viewBottom = viewTop + scrollRoot.clientHeight;

		if (markTop < viewTop + padding) {
			scrollRoot.scrollTop = Math.max(0, markTop - padding);
			return;
		}

		if (markBottom > viewBottom - padding) {
			scrollRoot.scrollTop = Math.max(0, markBottom - scrollRoot.clientHeight + padding);
		}
	}

	private wrapValueTermsInTextNodes(node: Node, valueTerms: string[]): HTMLElement[] {
		const marks: HTMLElement[] = [];

		if (node.nodeType === Node.TEXT_NODE) {
			if (!this.isJsonValueTextNode(node)) return marks;

			const text = node.textContent || '';
			if (!text) return marks;

			const inner = text.replace(/^"|"$/g, '');
			const matchingTerm = valueTerms.find(term => inner.toLowerCase() === term.toLowerCase());
			if (!matchingTerm) return marks;

			this.wrapExactTextMatch(node, text, matchingTerm, marks);
			return marks;
		}

		if (node.nodeType !== Node.ELEMENT_NODE) return marks;

		const el = node as Element;
		if (el.tagName === 'MARK' && el.classList.contains('search-hit')) return marks;

		Array.from(node.childNodes).forEach(child => {
			marks.push(...this.wrapValueTermsInTextNodes(child, valueTerms));
		});

		return marks;
	}

	private highlightMatchingPropertyKeys(valueMarks: Node[], keyTerms: string[]): void {
		const keySet = new Set(keyTerms.map(key => key.toLowerCase()));

		for (const valueMark of valueMarks) {
			const valueToken = this.findValueTokenElement(valueMark);
			if (!valueToken) continue;

			this.highlightKeyBeforeValueToken(valueToken, keySet);

			let cursor: Node | null = valueToken;
			let depth = 0;

			while (cursor && cursor.parentNode === this.codeEle.nativeElement) {
				cursor = cursor.previousSibling;
				if (!cursor) break;

				if (this.isClosingBracketNode(cursor)) {
					depth++;
					continue;
				}

				if (this.isOpeningBracketNode(cursor)) {
					if (depth === 0) {
						this.highlightKeyBeforeComposite(cursor, keySet);
						break;
					}
					depth--;
				}
			}
		}
	}

	private findValueTokenElement(valueMark: Node): Element | null {
		let current: Node | null = valueMark;
		while (current && current !== this.codeEle.nativeElement) {
			if (current.nodeType === Node.ELEMENT_NODE) {
				const el = current as Element;
				if (el.classList.contains('token') && (el.classList.contains('string') || el.classList.contains('number') || el.classList.contains('boolean'))) {
					return el;
				}
			}
			current = current.parentNode;
		}
		return null;
	}

	private highlightKeyBeforeValueToken(valueToken: Element, keySet: Set<string>): void {
		let cursor: Node | null = valueToken.previousSibling;
		while (cursor) {
			if (this.isColonOperator(cursor)) {
				const keyNode = this.readKeyNodeBeforeOperator(cursor);
				if (keyNode) {
					this.highlightKeyNodeIfNeeded(keyNode, keySet);
				}
				return;
			}
			cursor = cursor.previousSibling;
		}
	}

	private highlightKeyBeforeComposite(openNode: Node, keySet: Set<string>): void {
		let cursor: Node | null = openNode.previousSibling;
		while (cursor) {
			if (this.isColonOperator(cursor)) {
				const keyNode = this.readKeyNodeBeforeOperator(cursor);
				if (keyNode) {
					this.highlightKeyNodeIfNeeded(keyNode, keySet);
				}
				return;
			}
			if (this.isOpeningBracketNode(cursor) || this.isClosingBracketNode(cursor)) {
				cursor = cursor.previousSibling;
				continue;
			}
			if (cursor.nodeType === Node.TEXT_NODE && !cursor.textContent?.trim()) {
				cursor = cursor.previousSibling;
				continue;
			}
			return;
		}
	}

	private isOpeningBracketNode(node: Node): boolean {
		return (
			node.nodeType === Node.ELEMENT_NODE &&
			(node as Element).classList.contains('token') &&
			(node as Element).classList.contains('punctuation') &&
			/^[[{]$/.test((node.textContent || '').trim())
		);
	}

	private isClosingBracketNode(node: Node): boolean {
		return (
			node.nodeType === Node.ELEMENT_NODE &&
			(node as Element).classList.contains('token') &&
			(node as Element).classList.contains('punctuation') &&
			/^[\]}]$/.test((node.textContent || '').trim())
		);
	}

	private isColonOperator(node: Node): boolean {
		return (
			node.nodeType === Node.ELEMENT_NODE &&
			(node as Element).classList.contains('token') &&
			(node as Element).classList.contains('operator') &&
			(node.textContent || '').trim() === ':'
		);
	}

	private readKeyNodeBeforeOperator(operatorNode: Node): Node | null {
		let cursor = operatorNode.previousSibling;
		while (cursor) {
			if (cursor.nodeType === Node.TEXT_NODE && cursor.textContent?.trim()) {
				return cursor;
			}
			if (cursor.nodeType === Node.ELEMENT_NODE) {
				const el = cursor as Element;
				if (el.classList.contains('token') && el.classList.contains('property')) {
					return el.firstChild;
				}
			}
			cursor = cursor.previousSibling;
		}
		return null;
	}

	private readPropertyKeyName(keyNode: Node): string {
		const raw = keyNode?.textContent || '';
		return raw.trim().replace(/^"|"$/g, '');
	}

	private highlightKeyNodeIfNeeded(keyNode: Node, keySet: Set<string>): void {
		if (!keyNode || keyNode.nodeType !== Node.TEXT_NODE) return;
		if (keyNode.parentElement?.tagName === 'MARK') return;

		const text = keyNode.textContent || '';
		const keyName = this.readPropertyKeyName(keyNode);
		if (!keySet.has(keyName.toLowerCase())) return;

		this.wrapExactTextMatch(keyNode, text, keyName);
	}

	private isJsonValueTextNode(node: Node): boolean {
		const parent = node.parentElement;
		if (!parent?.classList.contains('token')) return false;
		return parent.classList.contains('string') || parent.classList.contains('number') || parent.classList.contains('boolean');
	}

	private wrapExactTextMatch(node: Node, text: string, match: string, marks?: HTMLElement[]): void {
		const matchIndex = text.toLowerCase().indexOf(match.toLowerCase());
		if (matchIndex === -1) return;

		const before = text.slice(0, matchIndex);
		const hit = text.slice(matchIndex, matchIndex + match.length);
		const after = text.slice(matchIndex + match.length);
		const parent = node.parentNode;
		if (!parent) return;

		if (before) {
			parent.insertBefore(document.createTextNode(before), node);
		}

		const mark = document.createElement('mark');
		mark.className = 'search-hit';
		mark.textContent = hit;
		parent.insertBefore(mark, node);
		marks?.push(mark);

		if (after) {
			parent.insertBefore(document.createTextNode(after), node);
		}

		parent.removeChild(node);
	}

	private escapeHtml(value: string): string {
		return value
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(/"/g, '&quot;')
			.replace(/'/g, '&#39;');
	}

	private escapeRegExp(value: string): string {
		return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
	}

	getHeaders() {
		if (this.type !== 'headers') return;
		let headers: any = [];
		const selectedHeaders = this.code;

		if (selectedHeaders)
			Object.entries(selectedHeaders).forEach(([key, value]) => {
				headers.push({
					header: key,
					value: Array.isArray(value) ? value[0] : value
				});
			});

		return {
			headersLength: headers.length,
			headers: this.showPayload ? headers : headers.slice(0, 6)
		};
	}

}
