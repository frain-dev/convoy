import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrismComponent } from './prism.component';
import 'prismjs/components/prism-javascript';
import 'prismjs/components/prism-yaml';
import 'prismjs/components/prism-scss';
import 'prismjs/components/prism-json';
import 'prismjs/plugins/line-numbers/prism-line-numbers';
import { ButtonComponent } from 'src/app/components/button/button.component';

@NgModule({
	declarations: [PrismComponent],
	imports: [CommonModule, ButtonComponent],
	exports: [PrismComponent]
})
export class PrismModule {}
