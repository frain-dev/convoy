import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import Markdoc from '@markdoc/markdoc';
import axios from 'axios';
import { ButtonComponent } from 'src/app/components/button/button.component';

@Component({
	selector: 'sdk-documentation',
	standalone: true,
	imports: [CommonModule, ButtonComponent],
	templateUrl: './sdk-documentation.component.html',
	styleUrls: ['./sdk-documentation.component.scss']
})
export class SdkDocumentationComponent implements OnInit {
	@Output() onAction = new EventEmitter<any>();
	tabs = [
		{ label: 'Javascript', id: 'js' },
		{ label: 'Python', id: 'python' },
		{ label: 'PHP', id: 'php' },
		{ label: 'Ruby', id: 'ruby' },
		{ label: 'Golang', id: 'go' }
	];
	activeTab = 'js';
	activeStep: 'Install and Configure' | 'Create Endpoint' | 'Send Event' = 'Install and Configure';
	documentation: any;
	steps: ['Install and Configure', 'Create Endpoint', 'Send Event'] = ['Install and Configure', 'Create Endpoint', 'Send Event'];

	constructor() {}

	ngOnInit() {
		this.switchTabs('js');
	}

	switchTabs(activeTab: string) {
		switch (activeTab) {
			case 'js':
				this.activeTab = 'js';
				this.renderDocumentation('/content/sdks/js/Installation.md');
				break;
			case 'python':
				this.activeTab = 'python';
				this.renderDocumentation('/content/sdks/python/Installation.md');
				break;
			case 'php':
				this.activeTab = 'php';
				this.renderDocumentation('/content/sdks/php/Installation.md');
				break;
			case 'ruby':
				this.activeTab = 'ruby';
				this.renderDocumentation('/content/sdks/ruby/Installation.md');
				break;
			case 'go':
				this.activeTab = 'go';
				this.renderDocumentation('/content/sdks/go/Installation.md');
				break;
			default:
				break;
		}

		this.activeStep = 'Install and Configure';
	}

	switchStep(activeTab: 'Install and Configure' | 'Create Endpoint' | 'Send Event') {
		switch (activeTab) {
			case 'Install and Configure':
				this.activeStep = 'Install and Configure';
				this.renderDocumentation('/content/sdks/' + this.activeTab + '/Installation' + '.md');
				break;
			case 'Create Endpoint':
				this.activeStep = 'Create Endpoint';
				this.renderDocumentation('/content/sdks/' + this.activeTab + '/Add Endpoint' + '.md');
				break;
			case 'Send Event':
				this.activeStep = 'Send Event';
				this.renderDocumentation('/content/sdks/' + this.activeTab + '/Send Event' + '.md');
				break;
			default:
				break;
		}
	}

	fetchDocumentation(mdContent: string) {
		return new Promise(async (resolve, reject) => {
			try {
				const http = await axios.create();
				const results = http.request({
					method: 'get',
					url: mdContent
				});
				resolve(results);
			} catch (error) {
				reject(error);
			}
		});
	}

	async renderDocumentation(mdContent: string) {
		const results: any = await this.fetchDocumentation(mdContent);
		const ast = Markdoc.parse(results.data);
		const content = Markdoc.transform(ast);
		this.documentation = Markdoc.renderers.html(content);
	}

	nextStep() {
		let stepIndex = this.steps.findIndex(step => step === this.activeStep) + 1;
		stepIndex === this.steps.length ? this.onAction.emit() : this.switchStep(this.steps[stepIndex]);
	}
}
