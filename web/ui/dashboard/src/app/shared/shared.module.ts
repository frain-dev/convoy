import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import 'prismjs/components/prism-javascript';
import 'prismjs/components/prism-yaml';
import 'prismjs/components/prism-scss';
import 'prismjs/components/prism-json';
import 'prismjs/plugins/line-numbers/prism-line-numbers';
import { SharedComponent } from './shared.component';
import { pipes } from './pipes';

@NgModule({
	declarations: [SharedComponent, ...pipes],
	imports: [CommonModule],
	exports: [SharedComponent, ...pipes]
})
export class SharedModule {}
