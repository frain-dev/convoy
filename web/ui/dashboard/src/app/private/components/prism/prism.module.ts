import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrismComponent } from './prism.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';

@NgModule({
	declarations: [PrismComponent],
	imports: [CommonModule, ButtonComponent, CopyButtonComponent],
	exports: [PrismComponent]
})
export class PrismModule {}
