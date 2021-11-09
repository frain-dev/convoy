import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import 'prismjs/components/prism-javascript';
import 'prismjs/components/prism-yaml';
import 'prismjs/components/prism-scss';
import 'prismjs/components/prism-json';
import 'prismjs/plugins/line-numbers/prism-line-numbers';
import { SharedComponent } from './shared.component';

@NgModule({
	declarations: [SharedComponent],
	imports: [CommonModule],
	exports: [SharedComponent]
})
export class SharedModule {}
