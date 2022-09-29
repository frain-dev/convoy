import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrismModule } from '../prism/prism.module';
import Markdoc from '@markdoc/markdoc';

@Component({
	selector: 'convoy-sdk-documentation',
	standalone: true,
	imports: [CommonModule, PrismModule],
	templateUrl: './sdk-documentation.component.html',
	styleUrls: ['./sdk-documentation.component.scss']
})
export class SdkDocumentationComponent implements OnInit {
	tabs = [
		{ label: 'Javascript', id: 'javascript' },
		{ label: 'Python', id: 'python' },
		{ label: 'PHP', id: 'php' },
		{ label: 'Ruby', id: 'ruby' },
		{ label: 'Golang', id: 'golang' }
	];
	activeTab = 'javascript';
	documentation: any;
	constructor() {}

	ngOnInit() {
		this.switchTabs('javascript');
	}

	switchTabs(activeTab: string) {
		switch (activeTab) {
			case 'javascript':
				this.activeTab = 'javascript';
				this.documentation = this.fetchDocumentation('./content/sdks/convoy-js.md');
				break;
			case 'python':
				this.activeTab = 'python';
				this.documentation = this.fetchDocumentation('./content/sdks/convoy-js.md');
				break;
			case 'php':
				this.activeTab = 'php';
				this.documentation = this.fetchDocumentation('./content/sdks/convoy-js.md');
				break;
			case 'ruby':
				this.activeTab = 'ruby';
				this.documentation = this.fetchDocumentation('./content/sdks/convoy-js.md');
				break;
			case 'golang':
				this.activeTab = 'golang';
				this.documentation = this.fetchDocumentation('./content/sdks/convoy-js.md');
				break;
			default:
				break;
		}
	}

	fetchDocumentation(mdContent: string) {
		const ast = Markdoc.parse(mdContent);

		const content = Markdoc.transform(ast);

		return Markdoc.renderers.html(content);
	}
}
